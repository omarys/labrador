package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/omarys/labrador/internal/domain"
)

var (
	reTrailingSpace = regexp.MustCompile(`(?m)[ \t]+$`)
	reBlankRuns     = regexp.MustCompile(`\n{3,}`)
)

// NormalizeText converts scraped chapter prose into stable Markdown source: LF
// line endings, no trailing whitespace, and paragraphs separated by exactly one
// blank line.
func NormalizeText(body string) string {
	text := strings.ReplaceAll(body, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = reTrailingSpace.ReplaceAllString(text, "")
	text = reBlankRuns.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

// WriteText writes a prose chapter to targetPath atomically as a complete
// Markdown document: a level-1 heading for the chapter title (omitted when the
// title is empty), one blank line, then the Markdown body and a trailing
// newline. The body is expected to be a Markdown fragment whose paragraphs are
// separated by one blank line.
func WriteText(targetPath string, _ domain.Series, chapter domain.Chapter, body string) error {
	text := NormalizeText(body)
	if text == "" {
		return fmt.Errorf("cannot write empty text chapter")
	}

	var b strings.Builder
	if title := markdownHeadingText(chapter.Title); title != "" {
		b.WriteString("# ")
		b.WriteString(title)
		b.WriteString("\n\n")
	}
	b.WriteString(text)
	b.WriteString("\n")

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	tmpPath := targetPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("writing temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("moving temp file to final path: %w", err)
	}
	return nil
}

// markdownHeadingText flattens a chapter title into a single heading line so a
// newline inside the title cannot break the Markdown heading structure.
func markdownHeadingText(title string) string {
	return strings.Join(strings.Fields(title), " ")
}
