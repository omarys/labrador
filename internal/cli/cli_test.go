package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/omarys/labrador/internal/cli"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/downloader"
	"github.com/omarys/labrador/internal/provider"
)

type cliMockProv struct {
	id      string
	name    string
	domain  string
	baseURL string
	series  []domain.Series
}

func (m *cliMockProv) ID() string   { return m.id }
func (m *cliMockProv) Name() string { return m.name }
func (m *cliMockProv) Capabilities() provider.Capabilities {
	return provider.Capabilities{CanSearch: true, CanBrowse: true}
}

func (m *cliMockProv) MatchesURL(rawURL string) bool {
	return strings.Contains(rawURL, m.domain)
}

func (m *cliMockProv) Search(_ context.Context, query string) ([]domain.Series, error) {
	return m.series, nil
}

func (m *cliMockProv) Browse(_ context.Context, _ provider.BrowseOptions) ([]domain.Series, error) {
	return m.series, nil
}

func (m *cliMockProv) GetTags(_ context.Context) ([]domain.Tag, error) {
	return nil, nil
}

func (m *cliMockProv) GetChapters(_ context.Context, _ domain.Series) ([]domain.Chapter, error) {
	num := 1.0
	return []domain.Chapter{
		{ID: "ch1", Title: "Chapter 1", Number: &num, URL: "http://mock/ch1"},
	}, nil
}

func (m *cliMockProv) GetPages(_ context.Context, _ domain.Chapter) ([]domain.Page, error) {
	pageURL := "http://mock/p0.jpg"
	if m.baseURL != "" {
		pageURL = m.baseURL + "/p0.jpg"
	}
	return []domain.Page{
		{Index: 0, URL: pageURL},
	}, nil
}

func TestCLI_Search_JSON(t *testing.T) {
	reg := provider.NewRegistry()
	_ = reg.Register(&cliMockProv{
		id:     "mangapill",
		name:   "MangaPill",
		domain: "mangapill.com",
		series: []domain.Series{
			{ID: "solo", Title: "Solo Leveling", URL: "https://mangapill.com/manga/solo"},
		},
	})

	dl := downloader.New(nil)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	app := &cli.App{
		Registry:   reg,
		Downloader: dl,
		Stdout:     stdout,
		Stderr:     stderr,
	}

	err := app.Run(context.Background(), []string{"search", "solo", "--json"})
	if err != nil {
		t.Fatalf("Search --json failed: %v", err)
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		t.Fatalf("Failed to parse stdout as JSON: %v (tdout: %s)", err, stdout.String())
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0]["title"] != "Solo Leveling" {
		t.Errorf("unexpected JSON item: %+v", results[0])
	}
}

func TestCLI_Fetch_JSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("\xFF\xD8\xFFdummy-jpg-bytes"))
	}))
	defer ts.Close()

	reg := provider.NewRegistry()
	_ = reg.Register(&cliMockProv{
		id:      "mangapill",
		name:    "MangaPill",
		domain:  "mangapill.com",
		baseURL: ts.URL,
		series: []domain.Series{
			{ID: "solo", Title: "Solo Leveling"},
		},
	})

	dl := downloader.New(ts.Client())
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	app := &cli.App{
		Registry:   reg,
		Downloader: dl,
		Stdout:     stdout,
		Stderr:     stderr,
	}

	tmpDir := t.TempDir()
	args := []string{
		"fetch",
		"--url", "https://mangapill.com/manga/solo",
		"--chapter", "1",
		"--output-dir", tmpDir,
		"--json",
	}

	err := app.Run(context.Background(), args)
	if err != nil {
		t.Fatalf("Fetch --json failed: %v (stderr: %s)", err, stderr.String())
	}

	var result downloader.DownloadResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse fetch result JSON: %v (stdout: %s)", err, stdout.String())
	}

	if result.ProviderID != "mangapill" || result.PageCount != 1 {
		t.Errorf("unexpected fetchresult: %+v", result)
	}
}

func TestCLI_Fetch_DeweyIntegration(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nfake-image-bytes"))
	}))
	defer ts.Close()

	reg := provider.NewRegistry()
	_ = reg.Register(&cliMockProv{
		id:      "mangadex",
		name:    "MangaDex",
		domain:  "mangadex.org",
		baseURL: ts.URL,
		series: []domain.Series{
			{ID: "solo", Title: "Solo Leveling", URL: "https://mangadex.org/title/solo"},
		},
	})

	dl := downloader.New(ts.Client())
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	app := &cli.App{
		Registry:   reg,
		Downloader: dl,
		Stdout:     stdout,
		Stderr:     stderr,
	}

	tmpDir := t.TempDir()
	// Dewey invocation: no --url and no --json, only --series and --chapter
	args := []string{
		"fetch",
		"--series", "Solo Leveling",
		"--chapter", "1",
		"--output-dir", tmpDir,
	}

	err := app.Run(context.Background(), args)
	if err != nil {
		t.Fatalf("Dewey headless fetch failed: %v (stderr: %s)", err, stderr.String())
	}

	// Dewey parses this JSON directly
	type DeweyPayload struct {
		FilePath       string  `json:"file_path"`
		PageCount      *int    `json:"page_count"`
		FetchURL       *string `json:"fetch_url"`
		SeriesFetchURL *string `json:"series_fetch_url"`
	}

	var deweyRes DeweyPayload
	if err := json.Unmarshal(stdout.Bytes(), &deweyRes); err != nil {
		t.Fatalf("Dewey failed to parse JSON output: %v (stdout: %s)", err, stdout.String())
	}

	if deweyRes.FilePath == "" {
		t.Errorf("expected file_path in Dewey payload")
	}
	if deweyRes.PageCount == nil || *deweyRes.PageCount != 1 {
		t.Errorf("expected page_count 1, got %+v", deweyRes.PageCount)
	}
	if deweyRes.FetchURL == nil || *deweyRes.FetchURL == "" {
		t.Errorf("expected fetch_url in Dewey payload")
	}
	if deweyRes.SeriesFetchURL == nil || *deweyRes.SeriesFetchURL == "" {
		t.Errorf("expected series_fetch_url in Dewey payload")
	}
}

func TestCLI_Fetch_SkipAndForce(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nfake-image-bytes"))
	}))
	defer ts.Close()

	reg := provider.NewRegistry()
	_ = reg.Register(&cliMockProv{
		id:      "mangapill",
		name:    "MangaPill",
		domain:  "mangapill.com",
		baseURL: ts.URL,
		series: []domain.Series{
			{ID: "solo", Title: "Solo Leveling"},
		},
	})

	dl := downloader.New(ts.Client())
	tmpDir := t.TempDir()

	// 1. Initial fetch
	stdout1 := &bytes.Buffer{}
	app1 := &cli.App{Registry: reg, Downloader: dl, Stdout: stdout1, Stderr: &bytes.Buffer{}}
	if err := app1.Run(context.Background(), []string{
		"fetch", "--url", "https://mangapill.com/manga/solo", "--chapter", "1", "--output-dir", tmpDir,
	}); err != nil {
		t.Fatalf("Initial fetch failed: %v", err)
	}
	if !strings.Contains(stdout1.String(), "Saved chapter to") {
		t.Errorf("expected initial fetch to save, got: %s", stdout1.String())
	}

	// 2. Second fetch - should skip
	stdout2 := &bytes.Buffer{}
	app2 := &cli.App{Registry: reg, Downloader: dl, Stdout: stdout2, Stderr: &bytes.Buffer{}}
	if err := app2.Run(context.Background(), []string{
		"fetch", "--url", "https://mangapill.com/manga/solo", "--chapter", "1", "--output-dir", tmpDir,
	}); err != nil {
		t.Fatalf("Second fetch failed: %v", err)
	}
	if !strings.Contains(stdout2.String(), "Skipped (already downloaded)") {
		t.Errorf("expected second fetch to be skipped, got: %s", stdout2.String())
	}

	// 3. Third fetch with --force - should re-download
	stdout3 := &bytes.Buffer{}
	app3 := &cli.App{Registry: reg, Downloader: dl, Stdout: stdout3, Stderr: &bytes.Buffer{}}
	if err := app3.Run(context.Background(), []string{
		"fetch", "--url", "https://mangapill.com/manga/solo", "--chapter", "1", "--output-dir", tmpDir, "--force",
	}); err != nil {
		t.Fatalf("Third fetch with --force failed: %v", err)
	}
	if !strings.Contains(stdout3.String(), "Saved chapter to") {
		t.Errorf("expected forced fetch to save, got: %s", stdout3.String())
	}
}

func TestCLI_Resume(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nfake-image-bytes"))
	}))
	defer ts.Close()

	reg := provider.NewRegistry()
	_ = reg.Register(&cliMockProv{
		id:      "mangadex",
		name:    "MangaDex",
		domain:  "mangadex.org",
		baseURL: ts.URL,
		series: []domain.Series{
			{ID: "solo", Title: "Solo Leveling", URL: "https://mangadex.org/title/solo"},
		},
	})

	dl := downloader.New(ts.Client())
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	tmpCache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpCache)

	app := &cli.App{
		Registry:   reg,
		Downloader: dl,
		Stdout:     stdout,
		Stderr:     stderr,
	}

	// 1. Resume when queue is empty
	if err := app.Run(context.Background(), []string{"resume"}); err != nil {
		t.Fatalf("resume on empty queue failed: %v", err)
	}

	// 2. Populate queue file and resume
	queueJSON := `[
		{
			"id": "item1",
			"provider_id": "mangadex",
			"provider_name": "MangaDex",
			"series": {"id": "solo", "title": "Solo Leveling"},
			"chapter": {"id": "ch1", "title": "Chapter 1"}
		}
	]`
	_ = os.MkdirAll(tmpCache+"/labrador", 0755)
	_ = os.WriteFile(tmpCache+"/labrador/queue.json", []byte(queueJSON), 0644)

	stdout.Reset()
	if err := app.Run(context.Background(), []string{"resume"}); err != nil {
		t.Fatalf("resume with items failed: %v", err)
	}

	if !strings.Contains(stdout.String(), "Resuming 1 download(s)") {
		t.Errorf("expected output to mention resuming 1 download, got: %s", stdout.String())
	}
}
