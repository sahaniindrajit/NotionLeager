package expense

import (
	"testing"
	"time"
)

func TestSubscriptionDateIn(t *testing.T) {
	tests := []struct {
		day   int
		year  int
		month time.Month
		want  string
	}{
		{1, 2026, time.September, "2026-09-01"},
		{30, 2026, time.September, "2026-09-30"},
		{30, 2026, time.October, "2026-10-30"},
		{30, 2026, time.February, "2026-02-28"}, // clamps to last day
		{30, 2028, time.February, "2028-02-29"}, // leap year
		{31, 2026, time.April, "2026-04-30"},
	}
	for _, tt := range tests {
		got := Subscription{Day: tt.day}.DateIn(tt.year, tt.month, time.UTC).Format("2006-01-02")
		if got != tt.want {
			t.Errorf("day %d in %d-%02d: got %s, want %s", tt.day, tt.year, tt.month, got, tt.want)
		}
	}
}

func TestMonthlySubscriptions(t *testing.T) {
	want := map[string]struct {
		amount float64
		day    int
	}{
		"YouTube Premium":    {50, 1},
		"Netflix":            {199, 30},
		"Google One (30 GB)": {59, 30},
		"Wispr Flow":         {400, 29},
		"OVH":                {1622, 3},
	}
	if len(MonthlySubscriptions) != len(want) {
		t.Fatalf("got %d subscriptions, want %d", len(MonthlySubscriptions), len(want))
	}
	for _, s := range MonthlySubscriptions {
		w, ok := want[s.Name]
		if !ok {
			t.Errorf("unexpected subscription %q", s.Name)
			continue
		}
		if s.Amount != w.amount || s.Day != w.day || s.Category != "Subscription" {
			t.Errorf("%s: got ₹%.0f on day %d (%s)", s.Name, s.Amount, s.Day, s.Category)
		}
	}
}
