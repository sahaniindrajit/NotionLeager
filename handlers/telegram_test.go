package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"notionLeager/notion"
)

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
	day29 := expenseDay(now, 29)

	reply := h.send("/addsubs")
	mustContain(t, reply, "✅ Added (₹2330.00)", "YouTube Premium — ₹50.00", "Netflix — ₹199.00", "Google One (30 GB) — ₹59.00",
		"Wispr Flow — ₹400.00", "OVH — ₹1622.00")

	want := map[string]expenseFields{
		"YouTube Premium":    {"YouTube Premium", 50, subsCatID, ym + "-01"},
		"Netflix":            {"Netflix", 199, subsCatID, day30},
		"Google One (30 GB)": {"Google One (30 GB)", 59, subsCatID, day30},
		"Wispr Flow":         {"Wispr Flow", 400, subsCatID, day29},
		"OVH":                {"OVH", 1622, subsCatID, ym + "-03"},
	}
	pages := h.notion.pagesIn(testDBID)
	if len(pages) != 5 {
		t.Fatalf("got %d pages, want 5: %+v", len(pages), pages)
	}
	for _, p := range pages {
		if p.expenseFields != want[p.Name] {
			t.Errorf("page %+v, want %+v", p.expenseFields, want[p.Name])
		}
	}

	// Running it again in the same month must not create duplicates.
	reply = h.send("/addsubs")
	mustContain(t, reply, "⏭ Already added")
	if strings.Contains(reply, "✅ Added") {
		t.Errorf("second run added expenses again:\n%s", reply)
	}
	if n := len(h.notion.pagesIn(testDBID)); n != 5 {
		t.Errorf("got %d pages after re-run, want 5", n)
	}
}

func TestAddSubsSpecificMonth(t *testing.T) {
	h := newHarness(t)

	reply := h.send("/addsubs 2027-02")
	mustContain(t, reply, "February 2027", "✅ Added (₹2330.00)")

	dates := map[string]string{}
	for _, p := range h.notion.pagesIn(testDBID) {
		dates[p.Name] = p.Date
	}
	if dates["YouTube Premium"] != "2027-02-01" || dates["Netflix"] != "2027-02-28" || dates["Google One (30 GB)"] != "2027-02-28" ||
		dates["Wispr Flow"] != "2027-02-28" || dates["OVH"] != "2027-02-03" {
		t.Errorf("unexpected dates for Feb 2027: %v", dates)
	}

	mustContain(t, h.send("/addsubs feb"), "Invalid month")
	if n := len(h.notion.pagesIn(testDBID)); n != 5 {
		t.Errorf("invalid month created pages: got %d, want 5", n)
	}
}

func TestSubsList(t *testing.T) {
	h := newHarness(t)
	mustContain(t, h.send("/subs"),
		"Monthly Subscriptions — ₹2330.00",
		"YouTube Premium — ₹50.00 on day 1",
		"Netflix — ₹199.00 on day 30",
		"Google One (30 GB) — ₹59.00 on day 30",
		"Wispr Flow — ₹400.00 on day 29",
		"OVH — ₹1622.00 on day 3")
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
	h.notion.add(testDBID, expenseFields{"Old dinner", 999, foodCatID, time.Now().AddDate(0, 0, -1).Format("2006-01-02")}, "")

	reply := h.send("/today")
	mustContain(t, reply, "Today", "₹570.00", "• Lunch — ₹450.00 (Food & Drink)", "• Coffee — ₹120.00 (Food & Drink)")
	if strings.Contains(reply, "Old dinner") {
		t.Errorf("/today included yesterday's expense:\n%s", reply)
	}
}

func TestExpenseStillParsed(t *testing.T) {
	h := newHarness(t)
	mustContain(t, h.send("Uber, 250, Travel, Office commute"), "✅ Saved", "Category: Travel")
	mustContain(t, h.send("/help"), "/today", "/categories", "/addsubs", "/withdraw", "/cash ", "/cashleft", "/cashmonth")
}

func expenseDay(now time.Time, day int) string {
	last := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
	if day > last {
		day = last
	}
	return time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
}
