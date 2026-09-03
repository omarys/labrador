package downloader_test

import (
	"archive/zip"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/downloader"
	"github.com/omarys/labrador/internal/provider"
)

type mockProv struct {
	id     string
	pages  []domain.Page
	delay  time.Duration
	active int32
	max    int32
}

func (m *mockProv) ID() string                          { return m.id }
func (m *mockProv) Name() string                        { return m.id }
func (m *mockProv) Capabilities() provider.Capabilities { return provider.Capabilities{} }
func (m *mockProv) MatchesURL(rawURL string) bool       { return true }
func (m *mockProv) Search(_ context.Context, _ string) ([]domain.Series, error) {
	return nil, nil
}

func (m *mockProv) Browse(_ context.Context, _ provider.BrowseOptions) ([]domain.Series, error) {
	return nil, nil
}

func (m *mockProv) GetTags(_ context.Context) ([]domain.Tag, error) {
	return nil, nil
}

func (m *mockProv) GetChapters(_ context.Context, _ domain.Series) ([]domain.Chapter, error) {
	return nil, nil
}

func (m *mockProv) GetPages(_ context.Context, _ domain.Chapter) ([]domain.Page, error) {
	cur := atomic.AddInt32(&m.active, 1)
	for {
		old := atomic.LoadInt32(&m.max)
		if cur <= old || atomic.CompareAndSwapInt32(&m.max, old, cur) {
			break
		}
	}

	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	atomic.AddInt32(&m.active, -1)
	return m.pages, nil
}

func TestDownloader_DownloadChapter_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return dummy PNG header bytes
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nfake-image-bytes"))
	}))
	defer ts.Close()

	prov := &mockProv{
		id: "testprov",
		pages: []domain.Page{
			{Index: 0, URL: ts.URL + "/page0.png"},
			{Index: 1, URL: ts.URL + "/page1.png"},
		},
	}

	dl := downloader.New(ts.Client())
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "test.cbz")

	series := domain.Series{ID: "solo", Title: "Solo Leveling"}
	chapter := domain.Chapter{ID: "ch1", Title: "Chapter 1"}

	res, err := dl.DownloadChapter(context.Background(), prov, series, chapter, downloader.DownloadOptions{
		OutputFile: outPath,
	})
	if err != nil {
		t.Fatalf("DownloadChapter failed: %v", err)
	}

	if res.PageCount != 2 {
		t.Errorf("expected 2 pages, got %d", res.PageCount)
	}
	if res.FilePath != outPath {
		t.Errorf("expected FilePath %s, got %s", outPath, res.FilePath)
	}

	// Verify zip contents
	r, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatalf("failed to open cbz: %v", err)
	}
	defer func() { _ = r.Close() }()

	if len(r.File) != 3 { // ComicInfo.xml + 000.png + 001.png
		t.Errorf("expected 3 files in archive, got %d", len(r.File))
	}
}

func TestDownloader_ProviderLocking_Serialization(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	}))
	defer ts.Close()

	prov := &mockProv{
		id: "same-prov",
		pages: []domain.Page{
			{Index: 0, URL: ts.URL + "/p0.png"},
		},
		delay: 50 * time.Millisecond,
	}

	dl := downloader.New(ts.Client())
	tmpDir := t.TempDir()

	done := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		idx := i
		go func() {
			_, _ = dl.DownloadChapter(context.Background(), prov, domain.Series{Title: "S"}, domain.Chapter{Title: "C"}, downloader.DownloadOptions{
				OutputFile: filepath.Join(tmpDir, filepath.Clean(string(rune('a'+idx)))+".cbz"),
			})
			done <- struct{}{}
		}()
	}

	<-done
	<-done

	// Verify that GetPages was executed sequentially (max concurrent = 1)
	if max := atomic.LoadInt32(&prov.max); max != 1 {
		t.Errorf("expected provider downloads to be serialized (max concurrency 1), got %d", max)
	}
}

func TestDownloader_ThrottleBackoff(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cnt := atomic.AddInt32(&attempts, 1)
		if cnt == 1 {
			// First request triggers HTTP 429 Too Many Requests
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		// Subsequent attempts succeed
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nfake-image-bytes"))
	}))
	defer ts.Close()

	prov := &mockProv{
		id: "throttled-prov",
		pages: []domain.Page{
			{Index: 0, URL: ts.URL + "/page0.png"},
		},
	}

	dl := downloader.New(ts.Client())
	if w := dl.ProviderWorkers(prov.ID()); w != 3 {
		t.Fatalf("expected initial workers to be 3, got %d", w)
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "throttled.cbz")

	_, err := dl.DownloadChapter(context.Background(), prov, domain.Series{Title: "S"}, domain.Chapter{Title: "C"}, downloader.DownloadOptions{
		OutputFile: outPath,
	})
	if err != nil {
		t.Fatalf("DownloadChapter should succeed after retry: %v", err)
	}

	// Verify that provider was backed off to 1 worker
	if !dl.IsProviderThrottled(prov.ID()) {
		t.Errorf("expected provider to be marked as throttled")
	}
	if w := dl.ProviderWorkers(prov.ID()); w != 1 {
		t.Errorf("expected provider workers to back off to 1, got %d", w)
	}
}
