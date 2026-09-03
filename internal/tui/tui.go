package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
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
