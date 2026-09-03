package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/downloader"
	"github.com/omarys/labrador/internal/provider"
)

type screen int

const (
	screenProviders screen = iota
	screenSearch
	screenBrowse
	screenChapters
	screenQueue
)

type tabIndex int

const (
	tabSearch tabIndex = iota
	tabBrowse
)

type QueueItemStatus string

const (
	StatusQueued      QueueItemStatus = "QUEUED"
	StatusDownloading QueueItemStatus = "DOWNLOADING"
	StatusCompleted   QueueItemStatus = "DONE"
	StatusFailed      QueueItemStatus = "FAILED"
)

type QueueItem struct {
	ID           string
	Provider     provider.Provider
	Series       domain.Series
	Chapter      domain.Chapter
	Status       QueueItemStatus
	ErrorMessage string
	AddedAt      time.Time
}

// Async messages
type searchResultMsg struct {
	results []searchResultItem
	err     error
}

type chaptersResultMsg struct {
	chapters []domain.Chapter
	err      error
	cacheKey string
}

type providerSearchChunkMsg struct {
	provider provider.Provider
	results  []domain.Series
	err      error
}

type queueDownloadFinishedMsg struct {
	itemID string
	err    error
}

type searchResultItem struct {
	Provider provider.Provider
	Series   domain.Series
}

type Model struct {
	registry   *provider.Registry
	downloader *downloader.Downloader
	ctx        context.Context
	cancelFunc context.CancelFunc

	screen         screen
	previousScreen screen
	activeTab      tabIndex
	width          int
	height         int
	spinner        spinner.Model
	textInput      textinput.Model
	isLoading      bool
	statusMsg      string

	// Provider screen state
	providers         []provider.Provider
	providerCursor    int
	selectedProviders map[string]bool // multi-select with space

	// Active provider for single-provider view
	activeProvider provider.Provider

	// Search & Browse state
	searchResults []searchResultItem
	searchCursor  int
	browseSort    domain.SortOrder // SortPopular or SortRecent

	// Chapter screen state
	activeSeries     domain.Series
	chapters         []domain.Chapter
	chapterCursor    int
	selectedChapters map[string]bool // chapters queued for download

	// Queue screen state
	queue         []*QueueItem
	queueCursor   int
	isDownloading bool
	hideCompleted bool

	// Select mode (for Dewey interactive URL resolution)
	selectMode  bool
	selectedURL string

	// Search streaming & cancellation
	searchCancel    context.CancelFunc
	pendingSearches int

	// In-memory chapter cache (providerID:seriesID -> chapters)
	chapterCache map[string][]domain.Chapter
}

func NewModel(reg *provider.Registry, dl *downloader.Downloader, ctx context.Context, cancel context.CancelFunc) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(draculaPink)

	ti := textinput.New()
	ti.Placeholder = "Type series title to search..."
	ti.Focus()

	providers := reg.All()

	// Default to active providers: mangakatana, mangadistrict, private_gallery, weebcentral
	preferred := map[string]bool{
		"mangakatana":     true,
		"mangadistrict":   true,
		"private_gallery": true,
		"weebcentral":     true,
	}
	selected := make(map[string]bool)
	for _, p := range providers {
		if preferred[p.ID()] {
			selected[p.ID()] = true
		}
	}

	return Model{
		registry:          reg,
		downloader:        dl,
		ctx:               ctx,
		cancelFunc:        cancel,
		screen:            screenProviders,
		previousScreen:    screenProviders,
		activeTab:         tabSearch,
		spinner:           s,
		textInput:         ti,
		providers:         providers,
		selectedProviders: selected,
		selectedChapters:  make(map[string]bool),
		browseSort:        domain.SortPopular,
		queue:             make([]*QueueItem, 0),
		chapterCache:      make(map[string][]domain.Chapter),
	}
}

// NewSelectModel initializes the model in selectMode searching user preferred providers for query.
func NewSelectModel(reg *provider.Registry, dl *downloader.Downloader, ctx context.Context, cancel context.CancelFunc, query string) Model {
	m := NewModel(reg, dl, ctx, cancel)
	m.selectMode = true
	m.screen = screenSearch
	m.activeTab = tabSearch
	m.textInput.SetValue(query)
	m.textInput.Blur()

	preferred := map[string]bool{
		"mangakatana":     true,
		"mangadistrict":   true,
		"private_gallery": true,
		"weebcentral":     true,
	}
	m.selectedProviders = make(map[string]bool)
	for _, p := range m.providers {
		if preferred[p.ID()] && p.Capabilities().CanSearch {
			m.selectedProviders[p.ID()] = true
		}
	}
	if len(m.selectedProviders) == 0 {
		for _, p := range m.providers {
			if p.Capabilities().CanSearch {
				m.selectedProviders[p.ID()] = true
			}
		}
	}

	m.isLoading = query != ""
	return m
}

type startQueueProcessingMsg struct{}

func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.isLoading || m.isDownloading {
		cmds = append(cmds, m.spinner.Tick)
	}
	if len(m.queue) > 0 {
		cmds = append(cmds, func() tea.Msg {
			return startQueueProcessingMsg{}
		})
	}
	if m.selectMode && m.textInput.Value() != "" {
		cmds = append(cmds, m.performSearchCmd(m.textInput.Value()))
	}
	return tea.Batch(cmds...)
}
