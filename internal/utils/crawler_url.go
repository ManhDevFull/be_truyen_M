package utils

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var chapterSuffixRegex = regexp.MustCompile(`(?i)/(chapter)[-_]?(\d+)(/)?$`)

// NormalizeCrawlerSourceURL strips a trailing /chapter-<num> from URL (if present)
// and returns the base URL with a detected target chapter number.
func NormalizeCrawlerSourceURL(raw string) (base string, target int) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0
	}

	u, err := url.Parse(raw)
	if err != nil {
		return raw, 0
	}

	path := strings.TrimSuffix(u.Path, "/")
	matches := chapterSuffixRegex.FindStringSubmatch(path)
	if len(matches) >= 3 {
		if num, err := strconv.Atoi(matches[2]); err == nil {
			target = num
		}
		path = chapterSuffixRegex.ReplaceAllString(path, "")
	}

	u.Path = path
	u.RawQuery = ""
	u.Fragment = ""

	base = strings.TrimSuffix(u.String(), "/")
	return base, target
}
