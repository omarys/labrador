package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/omarys/labrador/internal/archive"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
	"golang.org/x/sync/errgroup"
)

const (
	DefaultProviderWorkers   = 3 // 3 concurrent page workers per provider by default
	ThrottledProviderWorkers = 1 // Backed off to a single worker when throttled
	DefaultTimeout           = 30 * time.Second
	MaxRetries               = 3
)

// DownloadOptions configures destination paths for downloads.
type DownloadOptions struct {
	OutputDir  string // Directory to write <Series>/<Chapter>.cbz
	OutputFile string // Explicit file path (overrides OutputDir)
}

// DownloadResult returns machine-readable details for Dewey and the CLI.
type DownloadResult struct {
	ProviderID     string   `json:"provider_id"`
	SeriesID       string   `json:"series_id"`
	SeriesTitle    string   `json:"series_title"`
	SeriesURL      string   `json:"series_url"`
	SeriesFetchURL string   `json:"series_fetch_url,omitempty"`
	ChapterID      string   `json:"chapter_id"`
	ChapterTitle   string   `json:"chapter_title"`
	ChapterNumber  *float64 `json:"chapter_number,omitempty"`
	ChapterURL     string   `json:"chapter_url"`
	FetchURL       string   `json:"fetch_url,omitempty"`
	FilePath       string   `json:"file_path"`
	PageCount      int      `json:"page_count"`
}

// ProviderThrottleState manages dynamic worker concurrency and throttling per provider.
type ProviderThrottleState struct {
	mu            sync.RWMutex
	workers       int
	isThrottled   bool
	lastThrottled time.Time
}

func (s *ProviderThrottleState) Workers() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workers
}

func (s *ProviderThrottleState) IsThrottled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isThrottled
}

func (s *ProviderThrottleState) MarkThrottled() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workers = ThrottledProviderWorkers
	s.isThrottled = true
	s.lastThrottled = time.Now()
}

func (s *ProviderThrottleState) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workers = DefaultProviderWorkers
	s.isThrottled = false
}

// Downloader manages polite scraping, page fetching, dynamic worker pools, and archive creation.
type Downloader struct {
	client         *http.Client
	mu             sync.Mutex
	locks          map[string]*sync.Mutex
	throttleStates map[string]*ProviderThrottleState
}

// New creates a Downloader with per-provider worker management.
func New(client *http.Client) *Downloader {
	if client == nil {
		client = http.DefaultClient
	}
	return &Downloader{
		client:         client,
		locks:          make(map[string]*sync.Mutex),
		throttleStates: make(map[string]*ProviderThrottleState),
	}
}

// getLock returns the serialization mutex for a specific provider ID.
func (d *Downloader) getLock(providerID string) *sync.Mutex {
	d.mu.Lock()
	defer d.mu.Unlock()

	l, exists := d.locks[providerID]
	if !exists {
		l = &sync.Mutex{}
		d.locks[providerID] = l
	}
	return l
}

// GetProviderThrottleState returns or initializes the concurrency state for a provider.
func (d *Downloader) GetProviderThrottleState(providerID string) *ProviderThrottleState {
	d.mu.Lock()
	defer d.mu.Unlock()

	state, exists := d.throttleStates[providerID]
	if !exists {
		state = &ProviderThrottleState{
			workers: DefaultProviderWorkers,
		}
		d.throttleStates[providerID] = state
	}
	return state
}

// ProviderWorkers returns the active worker count for a provider (3 or 1).
func (d *Downloader) ProviderWorkers(providerID string) int {
	return d.GetProviderThrottleState(providerID).Workers()
}

// IsProviderThrottled returns true if the provider is currently backed off.
func (d *Downloader) IsProviderThrottled(providerID string) bool {
	return d.GetProviderThrottleState(providerID).IsThrottled()
}

// DownloadChapter fetches all pages for a chapter and writes a .cbz archive.
func (d *Downloader) DownloadChapter(
	ctx context.Context,
	prov provider.Provider,
	series domain.Series,
	chapter domain.Chapter,
	opts DownloadOptions,
) (*DownloadResult, error) {
	if prov == nil {
		return nil, fmt.Errorf("provider cannot be nil")
	}

	// 1. Acquire provider-level lock (serializes chapter downloads for this provider)
	lock := d.getLock(prov.ID())
	lock.Lock()
	defer lock.Unlock()

	throttleState := d.GetProviderThrottleState(prov.ID())
	workerCount := throttleState.Workers()

	// 2. Discover chapter pages
	pages, err := prov.GetPages(ctx, chapter)
	if err != nil {
		return nil, fmt.Errorf("getting chapter pages: %w", err)
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages found for chapter %s", chapter.ID)
	}

	// 3. Download page image bytes with a bounded per-provider worker pool
	pageDataList := make([]archive.PageData, len(pages))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(workerCount)

	for i, p := range pages {
		idx := i
		page := p
		g.Go(func() error {
			data, err := d.fetchPageWithRetry(gctx, prov.ID(), page)
			if err != nil {
				return fmt.Errorf("page %d (%s): %w", page.Index, page.URL, err)
			}
			pageDataList[idx] = data
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("downloading pages: %w", err)
	}

	// 4. Determine destination path
	destPath := opts.OutputFile
	if destPath == "" {
		outDir := opts.OutputDir
		if outDir == "" {
			home, _ := os.UserHomeDir()
			outDir = filepath.Join(home, "Downloads", "Manga")
		}
		sanitizedSeries := sanitizeFilename(series.Title)
		sanitizedChapter := sanitizeFilename(chapter.Title)
		if sanitizedChapter == "" {
			sanitizedChapter = fmt.Sprintf("Chapter_%d", chapter.Index)
		}
		destPath = filepath.Join(outDir, sanitizedSeries, fmt.Sprintf("%s - %s.cbz", sanitizedSeries, sanitizedChapter))
	}

	// 5. Build .cbz archive
	if err := archive.WriteCBZ(destPath, series, chapter, pageDataList); err != nil {
		return nil, fmt.Errorf("writing CBZ: %w", err)
	}

	absPath, err := filepath.Abs(destPath)
	if err != nil {
		absPath = destPath
	}

	return &DownloadResult{
		ProviderID:     prov.ID(),
		SeriesID:       series.ID,
		SeriesTitle:    series.Title,
		SeriesURL:      series.URL,
		SeriesFetchURL: series.URL,
		ChapterID:      chapter.ID,
		ChapterTitle:   chapter.Title,
		ChapterNumber:  chapter.Number,
		ChapterURL:     chapter.URL,
		FetchURL:       chapter.URL,
		FilePath:       absPath,
		PageCount:      len(pageDataList),
	}, nil
}

// fetchPageWithRetry downloads a single image, dynamically backing off on rate limits.
func (d *Downloader) fetchPageWithRetry(ctx context.Context, providerID string, page domain.Page) (archive.PageData, error) {
	var lastErr error

	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return archive.PageData{}, ctx.Err()
			case <-time.After(time.Duration(attempt*250) * time.Millisecond):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, page.URL, nil)
		if err != nil {
			return archive.PageData{}, fmt.Errorf("creating page request: %w", err)
		}

		// Apply provider-supplied headers (e.g. Referer)
		for k, values := range page.Headers {
			for _, v := range values {
				req.Header.Add(k, v)
			}
		}

		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Detect HTTP throttling (429 Too Many Requests, 503 Service Unavailable)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			_ = resp.Body.Close()
			// Back off this provider to 1 worker
			d.GetProviderThrottleState(providerID).MarkThrottled()

			select {
			case <-ctx.Done():
				return archive.PageData{}, ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
			}
			lastErr = fmt.Errorf("throttled by provider (HTTP %d)", resp.StatusCode)
			continue
		}

		if resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("server returned status: %s", resp.Status)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return archive.PageData{}, fmt.Errorf("unexpected status: %s", resp.Status)
		}

		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		ext := detectExtension(page.URL, data)
		return archive.PageData{
			Index:     page.Index,
			Extension: ext,
			Data:      data,
		}, nil
	}

	return archive.PageData{}, fmt.Errorf("failed after %d attempts: %w", MaxRetries, lastErr)
}

func detectExtension(rawURL string, data []byte) string {
	// 1. Sniff magic bytes
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return ".jpg"
	}
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return ".png"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return ".webp"
	}

	// 2. Fallback to URL extension
	ext := strings.ToLower(filepath.Ext(rawURL))
	if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" || ext == ".gif" {
		return ext
	}

	return ".jpg"
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"//", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return strings.TrimSpace(replacer.Replace(name))
}
