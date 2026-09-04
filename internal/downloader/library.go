package downloader

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/omarys/labrador/internal/domain"
)

var reParenthetical = regexp.MustCompile(`\s*\([^)]*\)\s*`)

// ResolveDefaultLibraryDir discovers the user's comic/manga library directory.
// It checks DEWEY_LIBRARY_DIR, LABRADOR_LIBRARY_DIR, ~/.config/dewey/config.toml,
// and falls back to ~/Downloads/Manga.
func ResolveDefaultLibraryDir() string {
	if env := os.Getenv("DEWEY_LIBRARY_DIR"); env != "" {
		if fi, err := os.Stat(env); err == nil && fi.IsDir() {
			return env
		}
	}
	if env := os.Getenv("LABRADOR_LIBRARY_DIR"); env != "" {
		if fi, err := os.Stat(env); err == nil && fi.IsDir() {
			return env
		}
	}

	// Check ~/.config/dewey/config.toml
	home, err := os.UserHomeDir()
	if err == nil {
		configPath := filepath.Join(home, ".config", "dewey", "config.toml")
		if data, err := os.ReadFile(configPath); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "library_dir") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						val := strings.TrimSpace(parts[1])
						val = strings.Trim(val, `"'`)
						if val != "" {
							if fi, err := os.Stat(val); err == nil && fi.IsDir() {
								return val
							}
						}
					}
				}
			}
		}
	}

	if home != "" {
		return filepath.Join(home, "Downloads", "Manga")
	}
	return "."
}

// NormalizeSeriesTitle strips parentheticals like "(Official)" and non-alphanumerics
// for fuzzy directory matching.
func NormalizeSeriesTitle(s string) string {
	cleaned := reParenthetical.ReplaceAllString(s, "")
	var b strings.Builder
	for _, r := range strings.ToLower(cleaned) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FindSeriesDirectoryInLibrary looks for an existing directory for the given series title
// inside libraryDir (including .Other and subdirectories like Manga, Manhwa, Romance).
func FindSeriesDirectoryInLibrary(libraryDir string, seriesTitle string) string {
	if libraryDir == "" || seriesTitle == "" {
		return ""
	}
	normTarget := NormalizeSeriesTitle(seriesTitle)
	if normTarget == "" {
		return ""
	}

	var found string
	_ = filepath.WalkDir(libraryDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path == libraryDir {
			return nil
		}

		name := d.Name()
		if NormalizeSeriesTitle(name) == normTarget {
			found = path
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(libraryDir, path)
		if err == nil && strings.Count(rel, string(filepath.Separator)) >= 4 {
			return filepath.SkipDir
		}
		return nil
	})

	return found
}

// FindExistingChapterFile checks if a chapter is already downloaded in targetDir.
// It checks both the direct filename and scans targetDir for any existing .cbz or .zip
// archive whose parsed chapter number corresponds to this chapter.
func FindExistingChapterFile(targetDir string, series domain.Series, chapter domain.Chapter) string {
	if targetDir == "" {
		return ""
	}
	cleanDir := filepath.Clean(targetDir)
	sanitizedSeries := sanitizeFilename(series.Title)
	sanitizedChapter := sanitizeFilename(chapter.Title)
	if sanitizedChapter == "" {
		sanitizedChapter = "Chapter_" + string(rune(chapter.Index))
	}

	// 1. Direct path check
	exactFile := filepath.Join(cleanDir, sanitizedSeries+" - "+sanitizedChapter+".cbz")
	if fi, err := os.Stat(exactFile); err == nil && fi.Size() > 0 {
		return exactFile
	}

	// 2. Direct chapter filename check
	chapFile := filepath.Join(cleanDir, sanitizedChapter+".cbz")
	if fi, err := os.Stat(chapFile); err == nil && fi.Size() > 0 {
		return chapFile
	}

	// 3. Scan target directory for any .cbz/.zip matching this chapter number
	entries, err := os.ReadDir(cleanDir)
	if err != nil {
		return ""
	}

	targetNum := domain.ChapterNumber(chapter)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".cbz") && !strings.HasSuffix(lower, ".zip") {
			continue
		}

		fileNum := domain.ParseChapterNumberFromFilename(name)
		if fileNum != nil && math.Abs(*fileNum-targetNum) < 0.001 {
			info, err := entry.Info()
			if err == nil && info.Size() > 0 {
				return filepath.Join(cleanDir, name)
			}
		}
	}

	return ""
}
