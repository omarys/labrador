package tui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/provider"
)

// SavedQueueItem is the serialized representation of a download queue item.
type SavedQueueItem struct {
	ID           string          `json:"id"`
	ProviderID   string          `json:"provider_id"`
	ProviderName string          `json:"provider_name"`
	Series       domain.Series   `json:"series"`
	Chapter      domain.Chapter  `json:"chapter"`
	Status       QueueItemStatus `json:"status"`
	AddedAt      time.Time       `json:"added_at"`
	ErrorMessage string          `json:"error_message,omitempty"`
	OutputDir    string          `json:"output_dir,omitempty"`
	SeriesDir    string          `json:"series_dir,omitempty"`
}

func getCacheFilePath() string {
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			cacheDir = "/tmp/labrador"
		} else {
			cacheDir = filepath.Join(home, ".cache", "labrador")
		}
	} else {
		cacheDir = filepath.Join(cacheDir, "labrador")
	}
	_ = os.MkdirAll(cacheDir, 0755)
	return filepath.Join(cacheDir, "queue.json")
}

// DumpQueue persists non-completed queue items to ~/.cache/labrador/queue.json synchronously (for app shutdown).
func (m *Model) DumpQueue() error {
	toSave := make([]SavedQueueItem, 0, len(m.queue))
	for _, item := range m.queue {
		if item.Status != StatusCompleted {
			status := item.Status
			if status == StatusDownloading {
				status = StatusQueued
			}
			toSave = append(toSave, SavedQueueItem{
				ID:           item.ID,
				ProviderID:   item.Provider.ID(),
				ProviderName: item.Provider.Name(),
				Series:       item.Series,
				Chapter:      item.Chapter,
				Status:       status,
				AddedAt:      item.AddedAt,
				ErrorMessage: item.ErrorMessage,
				OutputDir:    item.OutputDir,
				SeriesDir:    item.SeriesDir,
			})
		}
	}

	cacheFile := getCacheFilePath()
	if len(toSave) == 0 {
		_ = os.Remove(cacheFile)
		return nil
	}

	data, err := json.Marshal(toSave)
	if err != nil {
		return err
	}

	return os.WriteFile(cacheFile, data, 0644)
}

// dumpQueueAsyncCmd persists queue state in a background goroutine so it never blocks the TUI loop.
func dumpQueueAsyncCmd(queue []*QueueItem) tea.Cmd {
	toSave := make([]SavedQueueItem, 0, len(queue))
	for _, item := range queue {
		if item.Status != StatusCompleted {
			status := item.Status
			if status == StatusDownloading {
				status = StatusQueued
			}
			toSave = append(toSave, SavedQueueItem{
				ID:           item.ID,
				ProviderID:   item.Provider.ID(),
				ProviderName: item.Provider.Name(),
				Series:       item.Series,
				Chapter:      item.Chapter,
				Status:       status,
				AddedAt:      item.AddedAt,
				ErrorMessage: item.ErrorMessage,
				OutputDir:    item.OutputDir,
				SeriesDir:    item.SeriesDir,
			})
		}
	}

	return func() tea.Msg {
		cacheFile := getCacheFilePath()
		if len(toSave) == 0 {
			_ = os.Remove(cacheFile)
			return nil
		}
		data, err := json.Marshal(toSave)
		if err == nil {
			_ = os.WriteFile(cacheFile, data, 0644)
		}
		return nil
	}
}

// LoadPersistedQueue restores queued items from ~/.cache/labrador/queue.json.
func (m *Model) LoadPersistedQueue() {
	cacheFile := getCacheFilePath()
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return
	}

	var saved []SavedQueueItem
	if err := json.Unmarshal(data, &saved); err != nil {
		return
	}

	for _, s := range saved {
		prov, ok := m.registry.Get(s.ProviderID)
		if !ok {
			// Fallback provider stub if not in registry
			prov = &fallbackProvider{id: s.ProviderID, name: s.ProviderName}
		}

		m.queue = append(m.queue, &QueueItem{
			ID:           s.ID,
			Provider:     prov,
			Series:       s.Series,
			Chapter:      s.Chapter,
			Status:       StatusQueued,
			AddedAt:      s.AddedAt,
			ErrorMessage: s.ErrorMessage,
			OutputDir:    s.OutputDir,
			SeriesDir:    s.SeriesDir,
		})
	}
}

type fallbackProvider struct {
	id   string
	name string
}

func (f *fallbackProvider) ID() string                          { return f.id }
func (f *fallbackProvider) Name() string                        { return f.name }
func (f *fallbackProvider) Capabilities() provider.Capabilities { return provider.Capabilities{} }
func (f *fallbackProvider) MatchesURL(_ string) bool            { return false }
func (f *fallbackProvider) Search(_ context.Context, _ string) ([]domain.Series, error) {
	return nil, nil
}
func (f *fallbackProvider) Browse(_ context.Context, _ provider.BrowseOptions) ([]domain.Series, error) {
	return nil, nil
}
func (f *fallbackProvider) GetTags(_ context.Context) ([]domain.Tag, error) { return nil, nil }
func (f *fallbackProvider) GetChapters(_ context.Context, _ domain.Series) ([]domain.Chapter, error) {
	return nil, nil
}
func (f *fallbackProvider) GetPages(_ context.Context, _ domain.Chapter) ([]domain.Page, error) {
	return nil, nil
}
