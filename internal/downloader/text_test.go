package downloader_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/downloader"
	"github.com/omarys/labrador/internal/provider"
)

// textProv is a prose provider: it declares text media and implements
// provider.TextProvider instead of returning page images.
type textProv struct {
	id   string
	body string
	err  error
}

func (p *textProv) ID() string   { return p.id }
func (p *textProv) Name() string { return p.id }

func (p *textProv) Capabilities() provider.Capabilities {
	return provider.Capabilities{Media: domain.MediaText}
}

func (p *textProv) MatchesURL(string) bool { return true }

func (p *textProv) Search(context.Context, string) ([]domain.Series, error) { return nil, nil }

func (p *textProv) Browse(context.Context, provider.BrowseOptions) ([]domain.Series, error) {
	return nil, nil
}

func (p *textProv) GetTags(context.Context) ([]domain.Tag, error) { return nil, nil }

func (p *textProv) GetChapters(context.Context, domain.Series) ([]domain.Chapter, error) {
	return nil, nil
}

func (p *textProv) GetPages(context.Context, domain.Chapter) ([]domain.Page, error) {
	return nil, os.ErrInvalid // must never be called for text media
}

func (p *textProv) GetChapterText(context.Context, domain.Chapter) (string, error) {
	return p.body, p.err
}

func TestDownloadChapter_TextMedia_WritesMarkdown(t *testing.T) {
	outDir := t.TempDir()
	prov := &textProv{id: "novel", body: "Paragraph one.\n\n\nParagraph two.\n"}
	dl := downloader.New(nil)

	series := domain.Series{ID: "s1", Title: "Some Series", URL: "https://example.com/book/some-series"}
	chapter := domain.Chapter{ID: "c1", Title: "Chapter 12", URL: series.URL + "/chapter-12"}

	res, err := dl.DownloadChapter(context.Background(), prov, series, chapter, downloader.DownloadOptions{OutputDir: outDir})
	if err != nil {
		t.Fatalf("DownloadChapter failed: %v", err)
	}

	wantPath := filepath.Join(outDir, "Some Series", "Some Series - Chapter 12.md")
	if res.FilePath != wantPath {
		t.Errorf("FilePath = %q, want %q", res.FilePath, wantPath)
	}
	if res.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1", res.PageCount)
	}
	if res.Skipped {
		t.Errorf("first download should not be skipped")
	}

	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("reading chapter file: %v", err)
	}
	want := "# Chapter 12\n\nParagraph one.\n\nParagraph two.\n"
	if string(data) != want {
		t.Fatalf("unexpected contents:\n got %q\nwant %q", string(data), want)
	}
}

func TestDownloadChapter_TextMedia_SkipsExisting(t *testing.T) {
	outDir := t.TempDir()
	prov := &textProv{id: "novel", body: "Body."}
	dl := downloader.New(nil)

	series := domain.Series{ID: "s1", Title: "Some Series"}
	chapter := domain.Chapter{ID: "c1", Title: "Chapter 12"}

	if _, err := dl.DownloadChapter(context.Background(), prov, series, chapter, downloader.DownloadOptions{OutputDir: outDir}); err != nil {
		t.Fatalf("first download failed: %v", err)
	}

	res, err := dl.DownloadChapter(context.Background(), prov, series, chapter, downloader.DownloadOptions{OutputDir: outDir})
	if err != nil {
		t.Fatalf("second download failed: %v", err)
	}
	if !res.Skipped {
		t.Errorf("second download should be skipped")
	}
	if res.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1", res.PageCount)
	}
}

func TestResolveDestinationPath_TextMedia(t *testing.T) {
	series := domain.Series{Title: "Some Series"}
	chapter := domain.Chapter{Title: "Chapter 12"}

	got := downloader.ResolveDestinationPath(series, chapter, downloader.DownloadOptions{
		OutputDir: "/tmp/lib",
		Media:     domain.MediaText,
	})
	want := filepath.Join("/tmp/lib", "Some Series", "Some Series - Chapter 12.md")
	if got != want {
		t.Errorf("ResolveDestinationPath = %q, want %q", got, want)
	}

	got = downloader.ResolveDestinationPath(series, chapter, downloader.DownloadOptions{OutputDir: "/tmp/lib"})
	want = filepath.Join("/tmp/lib", "Some Series", "Some Series - Chapter 12.cbz")
	if got != want {
		t.Errorf("image media path = %q, want %q", got, want)
	}
}

func TestFindExistingChapterFile_TextMedia(t *testing.T) {
	dir := t.TempDir()
	series := domain.Series{ID: "s1", Title: "Some Series"}
	chapter := domain.Chapter{ID: "c1", Title: "Chapter 12"}

	target := filepath.Join(dir, "Some Series - Chapter 12.md")
	if err := os.WriteFile(target, []byte("# Chapter 12\n\nBody.\n"), 0o644); err != nil {
		t.Fatalf("writing existing chapter file: %v", err)
	}

	if got := downloader.FindExistingChapterFile(dir, series, chapter); got != target {
		t.Errorf("FindExistingChapterFile = %q, want %q", got, target)
	}
}
