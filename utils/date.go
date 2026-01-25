package utils

import "time"

// Monday → Today
func ThisWeekRange(now time.Time) (time.Time, time.Time) {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday
	}

	start := time.Date(
		now.Year(),
		now.Month(),
		now.Day()-(weekday-1),
		0, 0, 0, 0,
		now.Location(),
	)

	end := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		23, 59, 59, 0,
		now.Location(),
	)

	return start, end
}

// First day of month → today
func ThisMonthRange(now time.Time) (time.Time, time.Time) {
	start := time.Date(
		now.Year(),
		now.Month(),
		1,
		0, 0, 0, 0,
		now.Location(),
	)

	end := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		23, 59, 59, 0,
		now.Location(),
	)

	return start, end
}
