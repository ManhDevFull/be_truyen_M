package utils

import "time"

// WeekdaysToMask converts list of weekdays (1=Mon ... 7=Sun) to bitmask.
func WeekdaysToMask(days []int) int {
	mask := 0
	for _, d := range days {
		if d < 1 || d > 7 {
			continue
		}
		mask |= 1 << (d - 1)
	}
	return mask
}

// MaskHasWeekday checks if a bitmask contains a weekday (based on time.Weekday).
func MaskHasWeekday(mask int, t time.Time) bool {
	wd := t.Weekday() // Sunday=0
	day := int(wd)
	if day == 0 {
		day = 7
	}
	bit := 1 << (day - 1)
	return mask&bit != 0
}
