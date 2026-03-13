package crawler

import "time"

func due(last *time.Time, now time.Time, intervalMinutes int) bool {
	if last == nil {
		return true
	}
	return now.Sub(*last) >= time.Duration(intervalMinutes)*time.Minute
}

func pickInterval(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func maxInt(a int, b int) int {
	if a >= b {
		return a
	}
	return b
}
