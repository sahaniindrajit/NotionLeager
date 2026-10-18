package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
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
	testCashDB  = "test-cash-db"
	subsCatID   = "ac58fe2f-92a4-4d67-90f8-c6ff3e5b8a8a" // Subscription
	foodCatID   = "48da7a44-30d1-4420-8b42-06de1a085516" // Food & Drink
	travelCatID = "47a3c7b1-ec42-414d-b9df-f0b8ffe11ef6" // Travel
)

type expenseFields struct {
	Name       string
	Amount     float64
	CategoryID string
	Date       string
}

type fakePage struct {
	expenseFields
	ID           string
	DB           string
	WithdrawalID string
	Summary      string
	Archived     bool
}

// fakeNotion implements the subset of the Notion API the bot uses:
// page creation and database queries with date/relation filters and sorts.
type fakeNotion struct {
	mu    sync.Mutex
	pages []fakePage // in creation order

	// distinctTimes gives each page its own created_time (one second apart)
	// instead of all sharing one minute.
	distinctTimes bool
}

type textItem struct {
	Text struct {
		Content string `json:"content"`
	} `json:"text"`
}

type relationItem struct {
	ID string `json:"id"`
}

type pageProps struct {
	Name     struct{ Title []textItem }        `json:"Name"`
	Amount   struct{ Number float64 }          `json:"Amount"`
	Category struct{ Relation []relationItem } `json:"Category"`
	Date     struct {
		Date struct{ Start string } `json:"date"`
	} `json:"Date"`
	Summary *struct {
		RichText []textItem `json:"rich_text"`
	} `json:"Summary"`
	Withdrawal *struct {
		Relation []relationItem `json:"relation"`
	} `json:"Withdrawal"`
}

func (f *fakeNotion) add(db string, e expenseFields, withdrawalID string) fakePage {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := fakePage{expenseFields: e, ID: fmt.Sprintf("page-%d", len(f.pages)+1), DB: db, WithdrawalID: withdrawalID}
	f.pages = append(f.pages, p)
	return p
}

func (f *fakeNotion) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer test-key" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch {
	case r.Method == "POST" && r.URL.Path == "/pages":
		f.createPage(w, r)
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/pages/"):
		f.getPage(w, strings.TrimPrefix(r.URL.Path, "/pages/"))
	case r.Method == "PATCH" && strings.HasPrefix(r.URL.Path, "/pages/"):
		f.updatePage(w, r, strings.TrimPrefix(r.URL.Path, "/pages/"))
	case r.Method == "POST" && (r.URL.Path == "/databases/"+testDBID+"/query" || r.URL.Path == "/databases/"+testCashDB+"/query"):
		f.query(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (f *fakeNotion) createPage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Parent struct {
			DatabaseID string `json:"database_id"`
		} `json:"parent"`
		Properties pageProps `json:"properties"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	db := req.Parent.DatabaseID
	p := req.Properties
	if (db != testDBID && db != testCashDB) || len(p.Name.Title) == 0 || len(p.Category.Relation) == 0 || p.Date.Date.Start == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Only the Cash Expenses database has a Withdrawal relation.
	if (p.Withdrawal != nil) != (db == testCashDB) {
		http.Error(w, "Withdrawal is not a property that exists", http.StatusBadRequest)
		return
	}

	withdrawalID := ""
	if p.Withdrawal != nil && len(p.Withdrawal.Relation) > 0 {
		withdrawalID = p.Withdrawal.Relation[0].ID
	}
	page := f.add(db, expenseFields{p.Name.Title[0].Text.Content, p.Amount.Number, p.Category.Relation[0].ID, p.Date.Date.Start}, withdrawalID)
	if p.Summary != nil && len(p.Summary.RichText) > 0 {
		f.mu.Lock()
		f.pages[len(f.pages)-1].Summary = p.Summary.RichText[0].Text.Content
		f.mu.Unlock()
	}
	json.NewEncoder(w).Encode(map[string]string{"object": "page", "id": page.ID})
}

// getPage returns a page like Notion does, including deleted ones (archived: true).
func (f *fakeNotion) getPage(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, p := range f.pages {
		if p.ID == id {
			out := f.pageJSON(p, i)
			out["archived"] = p.Archived
			json.NewEncoder(w).Encode(out)
			return
		}
	}
	http.Error(w, "not found", http.StatusNotFound)
}

// sameMinute makes every page share one created_time, like real Notion does for
// pages added within the same minute, so tests can't rely on it for ordering.
var sameMinute = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02T15:04:05.000Z")

func (f *fakeNotion) pageJSON(p fakePage, seq int) map[string]interface{} {
	props := map[string]interface{}{
		"Name":     map[string]interface{}{"title": []interface{}{map[string]interface{}{"plain_text": p.Name}}},
		"Amount":   map[string]interface{}{"number": p.Amount},
		"Category": map[string]interface{}{"relation": []interface{}{map[string]interface{}{"id": p.CategoryID}}},
		"Date":     map[string]interface{}{"date": map[string]interface{}{"start": p.Date}},
	}
	if p.DB == testCashDB {
		props["Withdrawal"] = map[string]interface{}{"relation": []interface{}{map[string]interface{}{"id": p.WithdrawalID}}}
	}
	if p.Summary != "" {
		props["Summary"] = map[string]interface{}{"rich_text": []interface{}{map[string]interface{}{"plain_text": p.Summary}}}
	}
	created := sameMinute
	if f.distinctTimes {
		created = time.Date(2026, 1, 1, 0, 0, seq, 0, time.UTC).Format("2006-01-02T15:04:05.000Z")
	}
	return map[string]interface{}{"id": p.ID, "created_time": created, "properties": props}
}

// updatePage handles archiving (delete) and the property edits /lastedit sends.
func (f *fakeNotion) updatePage(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Archived   bool `json:"archived"`
		Properties struct {
			Name *struct {
				Title []textItem `json:"title"`
			} `json:"Name"`
			Amount *struct {
				Number float64 `json:"number"`
			} `json:"Amount"`
			Summary *struct {
				RichText []textItem `json:"rich_text"`
			} `json:"Summary"`
			Description *json.RawMessage `json:"Description"`
		} `json:"properties"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Properties.Description != nil {
		http.Error(w, "Description is not a property that exists", http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.pages {
		p := &f.pages[i]
		if p.ID != id || p.Archived {
			continue
		}
		p.Archived = req.Archived
		if n := req.Properties.Name; n != nil && len(n.Title) > 0 {
			p.Name = n.Title[0].Text.Content
		}
		if a := req.Properties.Amount; a != nil {
			p.Amount = a.Number
		}
		if d := req.Properties.Summary; d != nil && len(d.RichText) > 0 {
			p.Summary = d.RichText[0].Text.Content
		}
		json.NewEncoder(w).Encode(map[string]string{"object": "page", "id": id})
		return
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func (f *fakeNotion) query(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filter struct {
			And []struct {
				Property string            `json:"property"`
				Date     map[string]string `json:"date"`
				Relation map[string]string `json:"relation"`
			} `json:"and"`
		} `json:"filter"`
		Sorts []struct {
			Property  string `json:"property"`
			Timestamp string `json:"timestamp"`
			Direction string `json:"direction"`
		} `json:"sorts"`
		PageSize int `json:"page_size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	db := map[string]string{"/databases/" + testDBID + "/query": testDBID, "/databases/" + testCashDB + "/query": testCashDB}[r.URL.Path]

	f.mu.Lock()
	var matched []fakePage
	seq := map[string]int{}
	for i, p := range f.pages {
		seq[p.ID] = i
		if p.DB != db || p.Archived {
			continue
		}
		ok := true
		for _, c := range req.Filter.And {
			switch {
			case c.Date["on_or_after"] != "":
				ok = ok && p.Date >= c.Date["on_or_after"]
			case c.Date["on_or_before"] != "":
				ok = ok && p.Date <= c.Date["on_or_before"]
			case c.Relation != nil && c.Property == "Category":
				ok = ok && p.CategoryID == c.Relation["contains"]
			case c.Relation != nil && c.Property == "Withdrawal":
				ok = ok && p.WithdrawalID == c.Relation["contains"]
			default:
				f.mu.Unlock()
				http.Error(w, "unsupported filter", http.StatusBadRequest)
				return
			}
		}
		if ok {
			matched = append(matched, p)
		}
	}
	f.mu.Unlock()

	sort.SliceStable(matched, func(i, j int) bool {
		for _, s := range req.Sorts {
			var a, b string
			if s.Property == "Date" {
				a, b = matched[i].Date, matched[j].Date
			} else if s.Timestamp == "created_time" {
				a, b = fmt.Sprintf("%08d", seq[matched[i].ID]), fmt.Sprintf("%08d", seq[matched[j].ID])
			}
			if a == b {
				continue
			}
			if s.Direction == "descending" {
				return a > b
			}
			return a < b
		}
		return false
	})
	if req.PageSize > 0 && len(matched) > req.PageSize {
		matched = matched[:req.PageSize]
	}

	results := []map[string]interface{}{}
	f.mu.Lock()
	for _, p := range matched {
		results = append(results, f.pageJSON(p, seq[p.ID]))
	}
	f.mu.Unlock()
	json.NewEncoder(w).Encode(map[string]interface{}{"results": results, "has_more": false})
}

// pagesIn returns the non-deleted pages in a database, in creation order.
func (f *fakeNotion) pagesIn(db string) []fakePage {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []fakePage
	for _, p := range f.pages {
		if p.DB == db && !p.Archived {
			out = append(out, p)
		}
	}
	return out
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
	return newHarnessWithCashDB(t, testCashDB)
}

func newHarnessWithCashDB(t *testing.T, cashDB string) *harness {
	h := &harness{t: t, notion: &fakeNotion{}, tg: &fakeTelegram{}}
	ns := httptest.NewServer(h.notion)
	ts := httptest.NewServer(h.tg)
	t.Cleanup(ns.Close)
	t.Cleanup(ts.Close)

	oldNotion, oldTG := notion.APIBaseURL, telegram.APIBaseURL
	notion.APIBaseURL, telegram.APIBaseURL = ns.URL, ts.URL
	t.Cleanup(func() { notion.APIBaseURL, telegram.APIBaseURL = oldNotion, oldTG })

	notionClient = notion.NewClient("test-key", testDBID, cashDB)
	clientOnce.Do(func() {}) // client already set; stop the handler from replacing it

	h.handler = TelegramWebhook(config.Config{
		TelegramOwnerId:  fmt.Sprint(testOwnerID),
		TelegramBotToken: "test-token",
		NotionAPIKey:     "test-key",
		NotionExpenseDB:  testDBID,
		NotionCashDB:     cashDB,
	})
	return h
}
