package archive_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/omarys/labrador/internal/archive"
	"github.com/omarys/labrador/internal/domain"
)

func TestNormalizeText(t *testing.T) {
	body := "  Chapter body line.  \r\n\r\n\r\n\r\nNext   paragraph.\r\n  \r\n"
	got := archive.NormalizeText(body)
	want := "Chapter body line.\n\nNext   paragraph."
	if got != want {
		t.Fatalf("NormalizeText mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestWriteText_Success(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Series - Chapter 1.md")

	series := domain.Series{ID: "s1", Title: "Series"}
	chapter := domain.Chapter{ID: "c1", Title: "Chapter 1"}

	if err := archive.WriteText(target, series, chapter, "First paragraph.\n\n\nSecond paragraph.\n"); err != nil {
		t.Fatalf("WriteText failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}

	want := "# Chapter 1\n\nFirst paragraph.\n\nSecond paragraph.\n"
	if string(data) != want {
		t.Fatalf("unexpected file contents:\n got %q\nwant %q", string(data), want)
	}

	if _, err := os.Stat(target + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temporary file was not cleaned up")
	}
}

func TestWriteText_EmptyBodyError(t *testing.T) {
	target := filepath.Join(t.TempDir(), "empty.md")
	if err := archive.WriteText(target, domain.Series{}, domain.Chapter{}, "   \n\n  "); err == nil {
		t.Fatal("expected an error for an empty chapter body")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("no file should be written for an empty body")
	}
}

func TestWriteText_NoTitleOmitsHeader(t *testing.T) {
	target := filepath.Join(t.TempDir(), "untitled.md")
	if err := archive.WriteText(target, domain.Series{}, domain.Chapter{}, "Just prose."); err != nil {
		t.Fatalf("WriteText failed: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	want := "Just prose.\n"
	if string(data) != want {
		t.Fatalf("unexpected contents:\n got %q\nwant %q", string(data), want)
	}
}

func TestWriteText_TitleNewlinesCollapsed(t *testing.T) {
	target := filepath.Join(t.TempDir(), "titled.md")
	chapter := domain.Chapter{ID: "c1", Title: "  Chapter\n  One  "}
	if err := archive.WriteText(target, domain.Series{}, chapter, "Body."); err != nil {
		t.Fatalf("WriteText failed: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	want := "# Chapter One\n\nBody.\n"
	if string(data) != want {
		t.Fatalf("unexpected contents:\n got %q\nwant %q", string(data), want)
	}
}
