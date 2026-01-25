package expense

import (
	"sort"
	"time"

	"notionLeager/notion"
)

type DayTotal struct {
	Date   time.Time
	Amount float64
}

type CategoryTotal struct {
	Category string
	Amount   float64
}

type Summary struct {
	Total       float64
	DailyAvg    float64
	HighestDay  float64
	LowestDay   float64
	TopCategory string
}

func AggregateByDay(rows []notion.ExpenseRow) []DayTotal {
	m := make(map[string]float64)

	for _, r := range rows {
		key := r.Date.Format("2006-01-02")
		m[key] += r.Amount
	}

	out := make([]DayTotal, 0, len(m))
	for k, v := range m {
		d, _ := time.Parse("2006-01-02", k)
		out = append(out, DayTotal{Date: d, Amount: v})
	}

	// O(n log n) sort instead of O(n^2) bubble sort
	sort.Slice(out, func(i, j int) bool {
		return out[i].Date.Before(out[j].Date)
	})

	return out
}

func AggregateByCategory(rows []notion.ExpenseRow) []CategoryTotal {
	m := make(map[string]float64)

	for _, r := range rows {
		category := r.Category
		if category == "" {
			category = "Miscellaneous"
		}
		m[category] += r.Amount
	}

	out := make([]CategoryTotal, 0, len(m))
	for k, v := range m {
		out = append(out, CategoryTotal{Category: k, Amount: v})
	}

	// O(n log n) sort instead of O(n^2) bubble sort
	sort.Slice(out, func(i, j int) bool {
		return out[i].Amount > out[j].Amount
	})

	return out
}

func BuildSummary(rows []notion.ExpenseRow) Summary {
	if len(rows) == 0 {
		return Summary{}
	}

	// Single pass for total and day aggregation
	dayMap := make(map[string]float64)
	catMap := make(map[string]float64)
	var total float64

	for _, r := range rows {
		total += r.Amount
		dayMap[r.Date.Format("2006-01-02")] += r.Amount
		cat := r.Category
		if cat == "" {
			cat = "Miscellaneous"
		}
		catMap[cat] += r.Amount
	}

	// Find highest/lowest day
	var highest, lowest float64
	first := true
	for _, amt := range dayMap {
		if first {
			highest, lowest = amt, amt
			first = false
			continue
		}
		if amt > highest {
			highest = amt
		}
		if amt < lowest {
			lowest = amt
		}
	}

	// Find top category
	topCategory := ""
	var topAmount float64
	for cat, amt := range catMap {
		if amt > topAmount {
			topAmount = amt
			topCategory = cat
		}
	}

	dailyAvg := total / float64(len(dayMap))

	return Summary{
		Total:       total,
		DailyAvg:    dailyAvg,
		HighestDay:  highest,
		LowestDay:   lowest,
		TopCategory: topCategory,
	}
}
