package expense

import "time"

// Subscription is a recurring expense charged on the same day every month.
type Subscription struct {
	Name     string
	Amount   float64
	Category string
	Day      int // day of month the charge lands on
}

// MonthlySubscriptions are added in one go by the /addsubs command.
var MonthlySubscriptions = []Subscription{
	{Name: "YouTube Premium", Amount: 50, Category: "Subscription", Day: 1},
	{Name: "Netflix", Amount: 199, Category: "Subscription", Day: 30},
	{Name: "Google One (30 GB)", Amount: 59, Category: "Subscription", Day: 30},
}

// DateIn returns the charge date for the given month. If the month is shorter
// than Day (e.g. the 30th in February), the last day of the month is used.
func (s Subscription) DateIn(year int, month time.Month, loc *time.Location) time.Time {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
	day := s.Day
	if day > lastDay {
		day = lastDay
	}
	if day < 1 {
		day = 1
	}
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}
