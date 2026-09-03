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

	p := tea.NewProgram(&m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if fm, ok := finalModel.(*Model); ok {
		_ = fm.DumpQueue()
	}
	return err
}

// RunSelect launches the TUI directly on the search screen across all providers,
// returning the chosen series URL when the user presses Enter.
func RunSelect(parentCtx context.Context, reg *provider.Registry, dl *downloader.Downloader, query string) (string, error) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	m := NewSelectModel(reg, dl, ctx, cancel, query)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}
	if fm, ok := finalModel.(*Model); ok {
		return fm.selectedURL, nil
	}
	return "", nil
}
