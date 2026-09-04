package domain

import (
	"regexp"
	"strconv"
	"strings"
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

// ChapterNumber returns the effective numeric chapter value for comparison.
func ChapterNumber(ch Chapter) float64 {
	if ch.Number != nil {
		return *ch.Number
	}
	if n := ParseChapterNumber(ch.Title); n != nil {
		return *n
	}
	if n := ParseChapterNumber(ch.OriginalLabel); n != nil {
		return *n
	}
	return float64(ch.Index)
}

var reBracketPrefix = regexp.MustCompile(`^\[([0-9]+)\]_`)

// ParseChapterNumberFromFilename extracts the chapter number from a cbz/zip filename,
// handling Dewey and standard formats like:
// "[0001]_Chapter_99_-[End].cbz" -> 80.0
// "[0099]_Chapter_1.cbz" -> 1.0
// "Berserk - Chapter 9 - The Golden Age.cbz" -> 1.0
// "Solo Leveling - c105.cbz" -> 105.0
// "Chapter 12.5.cbz" -> 12.5
// "[0045].cbz" -> 45.0
func ParseChapterNumberFromFilename(filename string) *float64 {
	dot := strings.LastIndex(filename, ".")
	stem := filename
	if dot >= 0 {
		stem = filename[:dot]
	}

	// 1. Explicit chapter match (e.g. Chapter_80, Chapter 1, Episode 10, c105)
	if m := reExplicitChapter.FindStringSubmatch(stem); len(m) > 1 {
		if n, err := strconv.ParseFloat(m[1], 64); err == nil {
			return &n
		}
	}

	// 2. Remove bracket prefix index (e.g. [0001]_) if present and try again
	stripped := reBracketPrefix.ReplaceAllString(stem, "")
	if m := reExplicitChapter.FindStringSubmatch(stripped); len(m) > 1 {
		if n, err := strconv.ParseFloat(m[1], 64); err == nil {
			return &n
		}
	}

	// 3. Fallback to bracket number if nothing else
	if m := reBracketPrefix.FindStringSubmatch(stem); len(m) > 1 {
		if n, err := strconv.ParseFloat(m[1], 64); err == nil {
			return &n
		}
	}

	// 4. Fallback number
	return ParseChapterNumber(stripped)
}
