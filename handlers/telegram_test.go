package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"notionLeager/config"
	"notionLeager/notion"
	"notionLeager/telegram"
)

const (
	testOwnerID = 42
	testDBID    = "test-db"
	subsCatID   = "ac58fe2f-92a4-4d67-90f8-c6ff3e5b8a8a" // Subscription
	foodCatID   = "48da7a44-30d1-4420-8b42-06de1a085516" // Food & Drink
)

type fakePage struct {
	Name       string
	Amount     float64
	CategoryID string
	Date       string
}

// fakeNotion implements the subset of the Notion API the bot uses.
type fakeNotion struct {
	mu    sync.Mutex
	pages []fakePage
}

func (f *fakeNotion) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer test-key" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case r.Method == "POST" && r.URL.Path == "/pages":
		var req struct {
			Parent struct {
				DatabaseID string `json:"database_id"`
			} `json:"parent"`
			Properties struct {
				Name struct {
					Title []struct {
						Text struct{ Content string } `json:"text"`
					} `json:"title"`
				}
				Amount   struct{ Number float64 }
				Category struct {
					Relation []struct{ ID string } `json:"relation"`
				}
				Date struct {
					Date struct{ Start string } `json:"date"`
				}
			} `json:"properties"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Parent.DatabaseID != testDBID {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		p := req.Properties
		f.pages = append(f.pages, fakePage{
			Name:       p.Name.Title[0].Text.Content,
			Amount:     p.Amount.Number,
			CategoryID: p.Category.Relation[0].ID,
			Date:       p.Date.Date.Start,
		})
		w.Write([]byte(`{"object":"page"}`))

	case r.Method == "POST" && r.URL.Path == "/databases/"+testDBID+"/query":
		var req struct {
			Filter struct {
				And []struct {
					Date map[string]string `json:"date"`
				} `json:"and"`
			} `json:"filter"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		from, to := "0000-00-00", "9999-99-99"
		for _, c := range req.Filter.And {
			if v, ok := c.Date["on_or_after"]; ok {
				from = v
			}
			if v, ok := c.Date["on_or_before"]; ok {
				to = v
			}
		}
		results := []map[string]interface{}{}
		for i, p := range f.pages {
			if p.Date < from || p.Date > to {
				continue
			}
			results = append(results, map[string]interface{}{
				"id": fmt.Sprintf("page-%d", i),
				"properties": map[string]interface{}{
					"Name":     map[string]interface{}{"title": []interface{}{map[string]interface{}{"plain_text": p.Name}}},
					"Amount":   map[string]interface{}{"number": p.Amount},
					"Category": map[string]interface{}{"relation": []interface{}{map[string]interface{}{"id": p.CategoryID}}},
					"Date":     map[string]interface{}{"date": map[string]interface{}{"start": p.Date}},
				},
			})
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"results": results, "has_more": false})

	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (f *fakeNotion) snapshot() []fakePage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakePage(nil), f.pages...)
}

// fakeTelegram records messages the bot sends.
type fakeTelegram struct {
	mu   sync.Mutex
	msgs []string
}

func (f *fakeTelegram) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req telegram.SendMessageRequest
	json.NewDecoder(r.Body).Decode(&req)
	f.mu.Lock()
	f.msgs = append(f.msgs, req.Text)
	f.mu.Unlock()
	w.Write([]byte(`{"ok":true}`))
}

func (f *fakeTelegram) last(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.msgs) == 0 {
		t.Fatal("bot sent no message")
	}
	return f.msgs[len(f.msgs)-1]
}

type harness struct {
	t       *testing.T
	notion  *fakeNotion
	tg      *fakeTelegram
	handler http.HandlerFunc
}

// nextMsgID is shared across tests because the webhook deduper is package-global.
var nextMsgID int

func newHarness(t *testing.T) *harness {
	h := &harness{t: t, notion: &fakeNotion{}, tg: &fakeTelegram{}}
	ns := httptest.NewServer(h.notion)
	ts := httptest.NewServer(h.tg)
	t.Cleanup(ns.Close)
	t.Cleanup(ts.Close)

	oldNotion, oldTG := notion.APIBaseURL, telegram.APIBaseURL
	notion.APIBaseURL, telegram.APIBaseURL = ns.URL, ts.URL
	t.Cleanup(func() { notion.APIBaseURL, telegram.APIBaseURL = oldNotion, oldTG })

	notionClient = notion.NewClient("test-key", testDBID)
	clientOnce.Do(func() {}) // client already set; stop the handler from replacing it

	h.handler = TelegramWebhook(config.Config{
		TelegramOwnerId:  fmt.Sprint(testOwnerID),
		TelegramBotToken: "test-token",
		NotionAPIKey:     "test-key",
		NotionExpenseDB:  testDBID,
	})
	return h
}

// send posts a Telegram update to the webhook and returns the bot's reply.
func (h *harness) send(text string) string {
	h.t.Helper()
	nextMsgID++
	update := map[string]interface{}{
		"update_id": nextMsgID,
		"message": map[string]interface{}{
			"message_id": nextMsgID,
			"text":       text,
			"from":       map[string]interface{}{"id": testOwnerID},
			"chat":       map[string]interface{}{"id": 7},
		},
	}
	body, _ := json.Marshal(update)
	rec := httptest.NewRecorder()
	h.handler(rec, httptest.NewRequest("POST", "/webhook", bytes.NewReader(body)))
	reply := h.tg.last(h.t)
	h.t.Logf("> %s\n%s", text, reply)
	return reply
}

func mustContain(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("reply missing %q\n--- reply ---\n%s", w, got)
		}
	}
}

func TestAddSubsCurrentMonth(t *testing.T) {
	h := newHarness(t)
	now := time.Now()
	ym := now.Format("2006-01")
	day30 := expenseDay(now, 30)

	reply := h.send("/addsubs")
	mustContain(t, reply, "✅ Added (₹308.00)", "YouTube Premium — ₹50.00", "Netflix — ₹199.00", "Google One (30 GB) — ₹59.00")

	want := map[string]fakePage{
		"YouTube Premium":    {"YouTube Premium", 50, subsCatID, ym + "-01"},
		"Netflix":            {"Netflix", 199, subsCatID, day30},
		"Google One (30 GB)": {"Google One (30 GB)", 59, subsCatID, day30},
	}
	pages := h.notion.snapshot()
	if len(pages) != 3 {
		t.Fatalf("got %d pages, want 3: %+v", len(pages), pages)
	}
	for _, p := range pages {
		if p != want[p.Name] {
			t.Errorf("page %+v, want %+v", p, want[p.Name])
		}
	}

	// Running it again in the same month must not create duplicates.
	reply = h.send("/addsubs")
	mustContain(t, reply, "⏭ Already added")
	if strings.Contains(reply, "✅ Added") {
		t.Errorf("second run added expenses again:\n%s", reply)
	}
	if n := len(h.notion.snapshot()); n != 3 {
		t.Errorf("got %d pages after re-run, want 3", n)
	}
}

func TestAddSubsSpecificMonth(t *testing.T) {
	h := newHarness(t)

	reply := h.send("/addsubs 2027-02")
	mustContain(t, reply, "February 2027", "✅ Added (₹308.00)")

	dates := map[string]string{}
	for _, p := range h.notion.snapshot() {
		dates[p.Name] = p.Date
	}
	if dates["YouTube Premium"] != "2027-02-01" || dates["Netflix"] != "2027-02-28" || dates["Google One (30 GB)"] != "2027-02-28" {
		t.Errorf("unexpected dates for Feb 2027: %v", dates)
	}

	mustContain(t, h.send("/addsubs feb"), "Invalid month")
	if n := len(h.notion.snapshot()); n != 3 {
		t.Errorf("invalid month created pages: got %d, want 3", n)
	}
}

func TestSubsList(t *testing.T) {
	h := newHarness(t)
	mustContain(t, h.send("/subs"),
		"Monthly Subscriptions — ₹308.00",
		"YouTube Premium — ₹50.00 on day 1",
		"Netflix — ₹199.00 on day 30",
		"Google One (30 GB) — ₹59.00 on day 30")
}

func TestCategories(t *testing.T) {
	h := newHarness(t)
	reply := h.send("/categories")
	cats, _ := notion.SeedCategories()
	for _, c := range cats {
		mustContain(t, reply, "• "+c.Name)
	}
	mustContain(t, reply, "netflix", "swiggy")
}

func TestToday(t *testing.T) {
	h := newHarness(t)
	mustContain(t, h.send("/today"), "No expenses recorded.")

	mustContain(t, h.send("Lunch, 450, Food"), "✅ Saved")
	mustContain(t, h.send("Coffee, 120, cafe"), "✅ Saved")

	// An expense on another day must not show up.
	h.notion.mu.Lock()
	h.notion.pages = append(h.notion.pages, fakePage{"Old dinner", 999, foodCatID, time.Now().AddDate(0, 0, -1).Format("2006-01-02")})
	h.notion.mu.Unlock()

	reply := h.send("/today")
	mustContain(t, reply, "Today", "₹570.00", "• Lunch — ₹450.00 (Food & Drink)", "• Coffee — ₹120.00 (Food & Drink)")
	if strings.Contains(reply, "Old dinner") {
		t.Errorf("/today included yesterday's expense:\n%s", reply)
	}
}

func TestExpenseStillParsed(t *testing.T) {
	h := newHarness(t)
	mustContain(t, h.send("Uber, 250, Travel, Office commute"), "✅ Saved", "Category: Travel")
	mustContain(t, h.send("/help"), "/today", "/categories", "/addsubs")
}

func expenseDay(now time.Time, day int) string {
	last := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
	if day > last {
		day = last
	}
	return time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
}
