package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/downloader"
	"github.com/omarys/labrador/internal/provider"
)

// Run starts the interactive Bubble Tea application.
func Run(parentCtx context.Context, reg *provider.Registry, dl *downloader.Downloader) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	m := NewModel(reg, dl, ctx, cancel)
	m.LoadPersistedQueue()

	p := tea.NewProgram(&m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if fm, ok := finalModel.(*Model); ok {
		_ = fm.DumpQueue()
	}
	return err
}

// SelectedSeriesInfo holds details of a series picked in Select mode.
type SelectedSeriesInfo struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Provider string `json:"provider,omitempty"`
}

// RunSelect launches the TUI directly on the search screen across all providers,
// returning the chosen series information when the user presses Enter.
func RunSelect(parentCtx context.Context, reg *provider.Registry, dl *downloader.Downloader, query string) (*SelectedSeriesInfo, error) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	m := NewSelectModel(reg, dl, ctx, cancel, query)
	p := tea.NewProgram(&m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	if fm, ok := finalModel.(*Model); ok && fm.selectedURL != "" {
		title := fm.selectedURL
		if fm.selectedSeries != nil && fm.selectedSeries.Title != "" {
			title = fm.selectedSeries.Title
		}
		return &SelectedSeriesInfo{
			Title:    title,
			URL:      fm.selectedURL,
			Provider: fm.selectedProvider,
		}, nil
	}
	return nil, nil
}

// RunChapters launches the TUI directly on the chapter listing for the given series URL or title,
// allowing the user to select multiple chapters and queue downloads.
func RunChapters(parentCtx context.Context, reg *provider.Registry, dl *downloader.Downloader, seriesURL string, seriesTitle string, outputDir string) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	m := NewModel(reg, dl, ctx, cancel)
	m.outputDir = outputDir
	m.directChaptersMode = true
	m.LoadPersistedQueue()

	p, ok := reg.FindByURL(seriesURL)
	if !ok && seriesTitle != "" {
		for _, prov := range reg.List() {
			if prov.Capabilities().CanSearch {
				res, err := prov.Search(ctx, seriesTitle)
				if err == nil && len(res) > 0 {
					p = prov
					seriesURL = res[0].URL
					seriesTitle = res[0].Title
					break
				}
			}
		}
	}

	if p != nil {
		m.activeProvider = p
		m.activeSeries = domain.Series{URL: seriesURL, Title: seriesTitle}
		m.screen = screenChapters
		m.chapterCursor = 0
		m.selectedChapters = make(map[string]bool)
		m.isLoading = true
	}

	prog := tea.NewProgram(&m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := prog.Run()
	if fm, ok := finalModel.(*Model); ok {
		_ = fm.DumpQueue()
	}
	return err
}
