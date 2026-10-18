package handlers

import (
	"testing"
	"time"
)

// The "last" commands act on whatever was added most recently, in either table,
// even when an older expense has a later date (like a subscription dated the 30th).
func TestLastCommandsUseNewestAcrossTables(t *testing.T) {
	h := newHarness(t)
	future := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	h.notion.add(testDBID, expenseFields{"Netflix", 199, subsCatID, future}, "")

	mustContain(t, h.send("/withdraw 500"), "Withdrawal saved")
	mustContain(t, h.send("/cash Tea, 20, Food"), "Cash spend saved")

	mustContain(t, h.send("/last"), "Last Expense (cash)", "Name: Tea", "₹20.00")

	// Deleting removes the cash spend, not the withdrawal or the future-dated Netflix row.
	mustContain(t, h.send("/lastdelete"), "Deleted last expense (cash)", "Tea — ₹20.00")
	if n := len(h.notion.pagesIn(testCashDB)); n != 0 {
		t.Fatalf("cash spend not deleted: %d left", n)
	}
	if n := len(h.notion.pagesIn(testDBID)); n != 2 {
		t.Fatalf("main table changed: %d pages, want 2", n)
	}

	// Next newest is the withdrawal.
	mustContain(t, h.send("/lastdelete"), "Deleted last expense:", "Cash withdrawal — ₹500.00")
	main := h.notion.pagesIn(testDBID)
	if len(main) != 1 || main[0].Name != "Netflix" {
		t.Fatalf("want only Netflix left, got %+v", main)
	}
}

func TestLastEditCashSpendAndDescription(t *testing.T) {
	h := newHarness(t)
	h.send("/withdraw 500")
	h.send("/cash Tea, 20, Food")

	mustContain(t, h.send("/lastedit"), "Editing (cash): Tea — ₹20.00")
	mustContain(t, h.send("Amount"), "Enter new Amount")
	mustContain(t, h.send("25"), "updated successfully")

	mustContain(t, h.send("/lastedit"), "Editing (cash): Tea — ₹25.00")
	h.send("Description")
	mustContain(t, h.send("with biscuits"), "updated successfully")

	cash := h.notion.pagesIn(testCashDB)
	if len(cash) != 1 || cash[0].Amount != 25 || cash[0].Summary != "with biscuits" {
		t.Fatalf("cash spend after edits: %+v", cash)
	}
	mustContain(t, h.send("/last"), "Description: with biscuits")
}

func TestLastWithoutCashDB(t *testing.T) {
	h := newHarnessWithCashDB(t, "")
	h.send("Lunch, 450, Food")
	mustContain(t, h.send("/last"), "Last Expense\n", "Name: Lunch")
	mustContain(t, h.send("/lastdelete"), "Deleted last expense:", "Lunch — ₹450.00")
	mustContain(t, h.send("/last"), "No expenses found")
}

// After a restart the bot has no memory of what it created, so it falls back to
// Notion's created_time across both tables.
func TestLastAfterRestartUsesCreatedTime(t *testing.T) {
	h := newHarness(t)
	h.notion.distinctTimes = true
	h.notion.add(testDBID, expenseFields{"Cash withdrawal", 500, "3e878c44-adcc-81ce-a7d8-fc83b6c056a1", time.Now().Format("2006-01-02")}, "")
	h.notion.add(testCashDB, expenseFields{"Tea", 20, foodCatID, time.Now().Format("2006-01-02")}, "page-1")
	future := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	h.notion.add(testDBID, expenseFields{"Netflix", 199, subsCatID, future}, "")

	mustContain(t, h.send("/last"), "Last Expense\n", "Name: Netflix")
	mustContain(t, h.send("/lastdelete"), "Netflix — ₹199.00")
	mustContain(t, h.send("/last"), "Last Expense (cash)", "Name: Tea")
}

// An expense deleted directly in Notion is skipped.
func TestLastSkipsPagesDeletedInNotion(t *testing.T) {
	h := newHarness(t)
	h.send("Lunch, 450, Food")
	h.send("Coffee, 120, Food")

	h.notion.mu.Lock()
	h.notion.pages[len(h.notion.pages)-1].Archived = true
	h.notion.mu.Unlock()

	mustContain(t, h.send("/last"), "Name: Lunch")
}
