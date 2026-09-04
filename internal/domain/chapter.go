package domain

import (
	"regexp"
	"strconv"
)

// Chapter describes a remotely hosted chapter discovered by a provider.
//
// All provider-supplied identifiers, labels, URLs, and headers are untrusted
// and must be validated before they reach the network or filesystem.
type Chapter struct {
	ID            string              `json:"id"`
	SeriesID      string              `json:"series_id"`
	Title         string              `json:"title"`
	URL           string              `json:"url"`
	OriginalLabel string              `json:"original_label"`
	Number        *float64            `json:"number,omitempty"`
	Index         int                 `json:"index"`
	Headers       map[string][]string `json:"headers,omitempty"`
}

var (
	reExplicitChapter = regexp.MustCompile(`(?i)(?:chapter|chap|episode|ep|\bch|\bc|\b#)[_\s\.\-]*([0-9]+(?:\.[0-9]+)?)`)
	reSeasonPrefix    = regexp.MustCompile(`(?i)\bseason\s*[0-9]+\b|\bs[0-9]+\b`)
	reFallbackNumber  = regexp.MustCompile(`\b([0-9]+(?:\.[0-9]+)?)\b`)
)

// ParseChapterNumber extracts the true chapter number from a title or label,
// ignoring season updates and headers (e.g. "Season 1 Finale - Chapter 40" -> 40.0).
func ParseChapterNumber(title string) *float64 {
	if m := reExplicitChapter.FindStringSubmatch(title); len(m) > 1 {
		if n, err := strconv.ParseFloat(m[1], 64); err == nil {
			return &n
		}
	}
	cleaned := reSeasonPrefix.ReplaceAllString(title, "")
	if m := reFallbackNumber.FindStringSubmatch(cleaned); len(m) > 1 {
		if n, err := strconv.ParseFloat(m[1], 64); err == nil {
			return &n
		}
	}
	return nil
}
