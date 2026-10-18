package handlers

import (
	"strings"
	"testing"
	"time"

	"notionLeager/notion"
)

func TestCashFlow(t *testing.T) {
	h := newHarness(t)
	today := time.Now().Format("2006-01-02")

	// No withdrawal yet: nothing to link a cash spend to.
	mustContain(t, h.send("/cash Tea, 20, Food"), "No cash withdrawal found", "/withdraw")
	if n := len(h.notion.pagesIn(testCashDB)); n != 0 {
		t.Fatalf("cash spend saved without a withdrawal: %d pages", n)
	}

	mustContain(t, h.send("/withdraw abc"), "Invalid amount")
	mustContain(t, h.send("/withdraw"), "Invalid amount")

	mustContain(t, h.send("/withdraw 500"), "Withdrawal saved — ₹500.00")
	main := h.notion.pagesIn(testDBID)
	if len(main) != 1 || main[0].expenseFields != (expenseFields{"Cash withdrawal", 500, notion.CashCategoryID, today}) {
		t.Fatalf("withdrawal page: %+v", main)
	}
	w1 := main[0].ID

	mustContain(t, h.send("/cash Tea, 20, Food"),
		"Cash spend saved", "Name: Tea", "Amount: ₹20.00", "Category: Food & Drink", "₹480.00 left")
	mustContain(t, h.send("/cash Auto, 60, auto rickshaw, to station"),
		"Category: Miscellaneous", "Description: to station", "₹420.00 left")
	mustContain(t, h.send("/cash Auto, 60, travel"), "Category: Travel", "₹360.00 left")

	cash := h.notion.pagesIn(testCashDB)
	if len(cash) != 3 {
		t.Fatalf("got %d cash pages, want 3", len(cash))
	}
	for _, p := range cash {
		if p.WithdrawalID != w1 || p.Date != today {
			t.Errorf("cash page %+v: want linked to %s on %s", p, w1, today)
		}
	}
	if cash[0].CategoryID != foodCatID || cash[2].CategoryID != travelCatID || cash[1].Summary != "to station" {
		t.Errorf("unexpected cash pages: %+v", cash)
	}
	// Cash spends never touch the main (bank) table.
	if n := len(h.notion.pagesIn(testDBID)); n != 1 {
		t.Errorf("main table has %d pages, want 1", n)
	}

	// A second withdrawal becomes the one new spends link to.
	mustContain(t, h.send("/withdraw 1000 weekend"), "₹1000.00")
	main = h.notion.pagesIn(testDBID)
	w2 := main[1].ID
	if main[1].Name != "Cash withdrawal — weekend" {
		t.Errorf("withdrawal name %q", main[1].Name)
	}
	mustContain(t, h.send("/cash Dinner, 400, food"), "From ₹1000.00", "₹600.00 left")
	if got := h.notion.pagesIn(testCashDB)[3].WithdrawalID; got != w2 {
		t.Errorf("new spend linked to %s, want latest withdrawal %s", got, w2)
	}

	reply := h.send("/cashleft")
	mustContain(t, reply, "Cash Left — ₹960.00",
		"₹1000.00 on", "₹400.00 spent, ₹600.00 left",
		"₹500.00 on", "₹140.00 spent, ₹360.00 left")

	reply = h.send("/cashmonth")
	mustContain(t, reply, "₹540.00", "Food & Drink", "₹420.00", "Travel", "Miscellaneous")

	reply = h.send("/today")
	mustContain(t, reply, "₹1500.00", "• Cash withdrawal — ₹500.00 (Cash)",
		"💵 Paid in cash — ₹540.00", "• Tea — ₹20.00 (Food & Drink)", "• Dinner — ₹400.00 (Food & Drink)")
}

func TestCashLeftHidesSpentOldWithdrawals(t *testing.T) {
	h := newHarness(t)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	old := h.notion.add(testDBID, expenseFields{"Cash withdrawal", 200, notion.CashCategoryID, yesterday}, "")
	h.notion.add(testCashDB, expenseFields{"Snacks", 200, foodCatID, yesterday}, old.ID)

	h.send("/withdraw 300")
	reply := h.send("/cashleft")
	mustContain(t, reply, "Cash Left — ₹300.00", "₹300.00 on", "₹0.00 spent, ₹300.00 left")
	if strings.Contains(reply, "₹200.00 on") {
		t.Errorf("fully spent withdrawal should be hidden:\n%s", reply)
	}
}

func TestManualWithdrawalCounts(t *testing.T) {
	h := newHarness(t)
	// Old habit of typing a withdrawal as a normal expense still lands in Cash.
	mustContain(t, h.send("Withdrawn, 500, withdraw"), "Category: Cash")
	mustContain(t, h.send("ATM, 1000, atm"), "Category: Cash")
	mustContain(t, h.send("/cash Tea, 20, food"), "From ₹1000.00", "₹980.00 left")
}

func TestCashNotConfigured(t *testing.T) {
	h := newHarnessWithCashDB(t, "")
	for _, cmd := range []string{"/cash Tea, 20, Food", "/cashleft", "/cashmonth"} {
		mustContain(t, h.send(cmd), "Cash tracking isn't set up")
	}
	// Withdrawals go to the main table, so they still work.
	mustContain(t, h.send("/withdraw 500"), "Withdrawal saved")
	mustContain(t, h.send("/today"), "• Cash withdrawal — ₹500.00 (Cash)")
}
