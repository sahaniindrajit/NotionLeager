package expense

import (
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

func AggregateByDay(rows []notion.ExpenseRow) []DayTotal {
	m := make(map[string]float64)

	for _, r := range rows {
		key := r.Date.Format("2006-01-02")
		m[key] += r.Amount
	}

	var out []DayTotal
	for k, v := range m {
		d, _ := time.Parse("2006-01-02", k)
		out = append(out, DayTotal{
			Date:   d,
			Amount: v,
		})
	}

	// sort by date ascending
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i].Date.After(out[j].Date) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}

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

	var out []CategoryTotal
	for k, v := range m {
		out = append(out, CategoryTotal{
			Category: k,
			Amount:   v,
		})
	}

	// sort by amount desc
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i].Amount < out[j].Amount {
				out[i], out[j] = out[j], out[i]
			}
		}
	}

	return out
}
