package downloader_test

import (
	"archive/zip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
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
	calls  int32
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
	atomic.AddInt32(&m.calls, 1)
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

func TestDownloader_ProviderWorkers_Concurrency(t *testing.T) {
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

	done := make(chan struct{}, 4)
	for i := 0; i < 4; i++ {
		idx := i
		go func() {
			_, _ = dl.DownloadChapter(context.Background(), prov, domain.Series{Title: "S"}, domain.Chapter{Title: "C"}, downloader.DownloadOptions{
				OutputFile: filepath.Join(tmpDir, filepath.Clean(string(rune('a'+idx)))+".cbz"),
			})
			done <- struct{}{}
		}()
	}

	for i := 0; i < 4; i++ {
		<-done
	}

	// Verify that provider allowed up to 3 concurrent workers, never exceeding 3
	if max := atomic.LoadInt32(&prov.max); max != 3 {
		t.Errorf("expected provider downloads to use 3 workers, got max concurrency %d", max)
	}
}

func TestDownloader_ProviderWorkers_Throttled_Serialized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	}))
	defer ts.Close()

	prov := &mockProv{
		id: "throttled-prov-serialized",
		pages: []domain.Page{
			{Index: 0, URL: ts.URL + "/p0.png"},
		},
		delay: 50 * time.Millisecond,
	}

	dl := downloader.New(ts.Client())
	dl.GetProviderThrottleState(prov.ID()).MarkThrottled()
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

	// Verify that throttled provider was serialized to 1 worker
	if max := atomic.LoadInt32(&prov.max); max != 1 {
		t.Errorf("expected throttled provider downloads to be serialized (max concurrency 1), got %d", max)
	}
}

func TestDownloader_ThrottleCooldownRecovery(t *testing.T) {
	origCooldown := downloader.ThrottleCooldown
	downloader.ThrottleCooldown = 50 * time.Millisecond
	defer func() { downloader.ThrottleCooldown = origCooldown }()

	dl := downloader.New(nil)
	provID := "recover-prov"

	if w := dl.ProviderWorkers(provID); w != 3 {
		t.Fatalf("expected initial workers to be 3, got %d", w)
	}

	dl.GetProviderThrottleState(provID).MarkThrottled()
	if w := dl.ProviderWorkers(provID); w != 1 {
		t.Fatalf("expected throttled workers to be 1, got %d", w)
	}
	if !dl.IsProviderThrottled(provID) {
		t.Fatalf("expected provider to be marked as throttled")
	}

	// Wait for cooldown to expire
	time.Sleep(60 * time.Millisecond)

	if !dl.IsProviderThrottled(provID) && dl.ProviderWorkers(provID) != 3 {
		t.Errorf("expected workers to recover to 3 after cooldown, got %d", dl.ProviderWorkers(provID))
	}
	if dl.IsProviderThrottled(provID) {
		t.Errorf("expected IsProviderThrottled to return false after cooldown")
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

func TestDownloader_DownloadChapter_SkipExisting(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	outPath := filepath.Join(tmpDir, "skip-test.cbz")

	series := domain.Series{ID: "solo", Title: "Solo Leveling"}
	chapter := domain.Chapter{ID: "ch1", Title: "Chapter 1"}

	// 1. First download - should download normally
	res1, err := dl.DownloadChapter(context.Background(), prov, series, chapter, downloader.DownloadOptions{
		OutputFile: outPath,
	})
	if err != nil {
		t.Fatalf("First DownloadChapter failed: %v", err)
	}
	if res1.Skipped {
		t.Errorf("expected first download to NOT be skipped")
	}
	if res1.PageCount != 2 {
		t.Errorf("expected 2 pages, got %d", res1.PageCount)
	}
	if atomic.LoadInt32(&prov.calls) != 1 {
		t.Errorf("expected 1 GetPages call, got %d", atomic.LoadInt32(&prov.calls))
	}

	// 2. Second download without Force - should be skipped without calling provider GetPages
	res2, err := dl.DownloadChapter(context.Background(), prov, series, chapter, downloader.DownloadOptions{
		OutputFile: outPath,
	})
	if err != nil {
		t.Fatalf("Second DownloadChapter failed: %v", err)
	}
	if !res2.Skipped {
		t.Errorf("expected second download to be skipped")
	}
	if res2.PageCount != 2 {
		t.Errorf("expected skipped download to report 2 pages from archive, got %d", res2.PageCount)
	}
	if atomic.LoadInt32(&prov.calls) != 1 {
		t.Errorf("expected GetPages calls to stay 1, got %d", atomic.LoadInt32(&prov.calls))
	}

	// 3. Third download with Force: true - should re-download
	res3, err := dl.DownloadChapter(context.Background(), prov, series, chapter, downloader.DownloadOptions{
		OutputFile: outPath,
		Force:      true,
	})
	if err != nil {
		t.Fatalf("Third DownloadChapter (Force) failed: %v", err)
	}
	if res3.Skipped {
		t.Errorf("expected forced download to NOT be skipped")
	}
	if atomic.LoadInt32(&prov.calls) != 2 {
		t.Errorf("expected GetPages calls to increment to 2, got %d", atomic.LoadInt32(&prov.calls))
	}
}

func TestDownloader_DownloadChapter_SkipDeweyBracketFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nfake-image-bytes"))
	}))
	defer ts.Close()

	prov := &mockProv{
		id: "testprov",
		pages: []domain.Page{
			{Index: 0, URL: ts.URL + "/page0.png"},
		},
	}

	dl := downloader.New(ts.Client())
	tmpDir := t.TempDir()

	// Create a simulated Dewey archive file: [0017]_Chapter_1_The_Wind_of_Swords.cbz
	deweyFile := filepath.Join(tmpDir, "[0017]_Chapter_1_The_Wind_of_Swords.cbz")
	if err := os.WriteFile(deweyFile, []byte("PK\x05\x06fake-zip"), 0644); err != nil {
		t.Fatalf("failed creating fake zip: %v", err)
	}

	series := domain.Series{ID: "berserk", Title: "Berserk"}
	chapter := domain.Chapter{ID: "ch1", Title: "Chapter 1 - The Wind of Swords"}

	res, err := dl.DownloadChapter(context.Background(), prov, series, chapter, downloader.DownloadOptions{
		SeriesDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("DownloadChapter failed: %v", err)
	}
	if !res.Skipped {
		t.Errorf("expected download to be skipped because [0017]_Chapter_1_The_Wind_of_Swords.cbz exists")
	}
	if res.FilePath != deweyFile {
		t.Errorf("expected res.FilePath to be %s, got %s", deweyFile, res.FilePath)
	}
	if atomic.LoadInt32(&prov.calls) != 0 {
		t.Errorf("expected 0 GetPages calls for skipped chapter, got %d", atomic.LoadInt32(&prov.calls))
	}
}
