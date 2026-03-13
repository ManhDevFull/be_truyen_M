package utils

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func Slugify(input string) string {
	value := strings.TrimSpace(strings.ToLower(input))
	if value == "" {
		return ""
	}

	value = strings.ReplaceAll(value, "đ", "d")
	value = strings.ReplaceAll(value, "Đ", "d")
	value = norm.NFD.String(value)

	var b strings.Builder
	b.Grow(len(value))
	lastDash := false
	for _, r := range value {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	return result
}
