package archive_test

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"path/filepath"
	"testing"

	"github.com/omarys/labrador/internal/archive"
	"github.com/omarys/labrador/internal/domain"
)

func TestGenerateComicInfo(t *testing.T) {
	num := 105.0
	series := domain.Series{Title: "Solo Leveling"}
	chapter := domain.Chapter{
		Title:  "Chapter 105",
		Number: &num,
		URL:    "https://example.com/solo/105",
	}

	data, err := archive.GenerateComicInfo(series, chapter, 48)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed archive.ComicInfo
	if err := xml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal generated XML: %v", err)
	}

	if parsed.Title != "Chapter 105" || parsed.Series != "Solo Leveling" {
		t.Errorf("unexpected parsed data: %+v", parsed)
	}
	if parsed.Number != "105" || parsed.PageCount != 48 {
		t.Errorf("unexpected number/page count: %+v", parsed)
	}
}

func TestWriteCBZ_Success(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "Solo Leveling", "Solo Leveling - Ch 105.cbz")

	num := 105.0
	series := domain.Series{Title: "Solo Leveling"}
	chapter := domain.Chapter{Title: "Chapter 105", Number: &num}

	pages := []archive.PageData{
		{Index: 0, Extension: ".jpg", Data: []byte("fake-jpeg-data-page-0")},
		{Index: 1, Extension: ".jpg", Data: []byte("fake-jpeg-data-page-1")},
	}

	if err := archive.WriteCBZ(targetPath, series, chapter, pages); err != nil {
		t.Fatalf("WriteCBZ failed: %v", err)
	}

	// Verify the zip file exists and entries are readable
	r, err := zip.OpenReader(targetPath)
	if err != nil {
		t.Fatalf("failed to open created cbz: %v", err)
	}
	defer func() { _ = r.Close() }()

	expectedEntries := map[string]string{
		"ComicInfo.xml": "",
		"000.jpg":       "fake-jpeg-data-page-0",
		"001.jpg":       "fake-jpeg-data-page-1",
	}

	foundCount := 0
	for _, f := range r.File {
		expectedContent, exists := expectedEntries[f.Name]
		if !exists {
			t.Errorf("unexpected file in archive: %s", f.Name)
			continue
		}
		foundCount++

		if expectedContent != "" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to open entry %s: %v", f.Name, err)
			}
			content, _ := io.ReadAll(rc)
			_ = rc.Close()
			if string(content) != expectedContent {
				t.Errorf("entry %s content mismatch: got %s, want %s", f.Name, string(content), expectedContent)
			}
		}
	}
}

func TestWriteCBZ_EmptyPagesError(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "empty.cbz")

	err := archive.WriteCBZ(targetPath, domain.Series{}, domain.Chapter{}, nil)
	if err == nil {
		t.Fatalf("expected error when writing empty archive, got nil")
	}
}
