package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/downloader"
	"github.com/omarys/labrador/internal/provider"
)

func (m *Model) filteredQueue() []*QueueItem {
	if !m.hideCompleted {
		return m.queue
	}
	out := make([]*QueueItem, 0, len(m.queue))
	for _, item := range m.queue {
		if item.Status != StatusCompleted {
			out = append(out, item)
		}
	}
	return out
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		if m.isLoading || m.isDownloading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case providerSearchChunkMsg:
		if m.pendingSearches > 0 {
			m.pendingSearches--
		}
		if m.pendingSearches == 0 {
			m.isLoading = false
		}

		if msg.err == nil && len(msg.results) > 0 {
			for _, s := range msg.results {
				m.searchResults = append(m.searchResults, searchResultItem{
					Provider: msg.provider,
					Series:   s,
				})
			}
			if m.pendingSearches > 0 {
				m.statusMsg = fmt.Sprintf("Found %d results (%d searching...)", len(m.searchResults), m.pendingSearches)
			} else {
				m.statusMsg = fmt.Sprintf("Found %d results", len(m.searchResults))
			}
		} else if len(m.searchResults) == 0 && m.pendingSearches == 0 {
			m.statusMsg = "No series found."
		}

		if m.selectMode {
			m.textInput.Blur()
		}

	case searchResultMsg:
		m.isLoading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.searchResults = msg.results
			m.searchCursor = 0
			m.statusMsg = fmt.Sprintf("Found %d results", len(msg.results))
		}
		if m.selectMode {
			m.textInput.Blur()
		}

	case chaptersResultMsg:
		m.isLoading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.chapters = msg.chapters
			m.chapterCursor = 0
			m.selectedChapters = make(map[string]bool)
			m.statusMsg = fmt.Sprintf("Found %d chapters", len(msg.chapters))
			if msg.cacheKey != "" {
				m.chapterCache[msg.cacheKey] = msg.chapters
			}
		}

	case queueDownloadFinishedMsg:
		for _, item := range m.queue {
			if item.ID == msg.itemID {
				if msg.err != nil {
					item.Status = StatusFailed
					item.ErrorMessage = msg.err.Error()
					m.statusMsg = fmt.Sprintf("Download failed: %s (%v)", item.Chapter.Title, msg.err)
				} else {
					item.Status = StatusCompleted
					m.statusMsg = fmt.Sprintf("Downloaded: %s (%s)", item.Chapter.Title, item.Series.Title)
				}
				break
			}
		}
		nextModel, nextCmd := m.startNextDownload()
		return nextModel, tea.Batch(dumpQueueAsyncCmd(m.queue), nextCmd)

	case startQueueProcessingMsg:
		return m.startNextDownload()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.cancelFunc != nil {
				m.cancelFunc()
			}
			_ = m.DumpQueue()
			return m, tea.Quit

		case "Q":
			// Toggle into/out of Download Queue screen
			if m.screen != screenQueue {
				m.previousScreen = m.screen
				m.screen = screenQueue
				m.queueCursor = 0
			} else {
				m.screen = m.previousScreen
			}
			return m, nil

		case "q":
			if !m.textInput.Focused() {
				if m.selectMode {
					return m, tea.Quit
				}
				if m.screen == screenProviders {
					if m.cancelFunc != nil {
						m.cancelFunc()
					}
					_ = m.DumpQueue()
					return m, tea.Quit
				}
				if m.screen == screenQueue {
					m.screen = m.previousScreen
					return m, nil
				}
				// Go back one screen
				return m.handleBack()
			}

		case "esc":
			if m.textInput.Focused() {
				m.textInput.Blur()
			} else if m.selectMode {
				return m, tea.Quit
			} else if m.screen == screenQueue {
				m.screen = m.previousScreen
				return m, nil
			} else {
				return m.handleBack()
			}

		default:
			switch m.screen {
			case screenProviders:
				return m.updateProviders(msg)
			case screenSearch, screenBrowse:
				return m.updateSearchBrowse(msg)
			case screenChapters:
				return m.updateChapters(msg)
			case screenQueue:
				return m.updateQueue(msg)
			}
		}
	}

	if m.textInput.Focused() {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleBack() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenChapters:
		if m.activeTab == tabBrowse {
			m.screen = screenBrowse
		} else {
			m.screen = screenSearch
		}
	case screenSearch, screenBrowse:
		m.screen = screenProviders
		m.searchResults = nil
	case screenQueue:
		m.screen = m.previousScreen
	}
	return m, nil
}

func (m *Model) updateProviders(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "J", "down", "ctrl+n":
		if m.providerCursor < len(m.providers)-1 {
			m.providerCursor++
		}
	case "k", "K", "up", "ctrl+p":
		if m.providerCursor > 0 {
			m.providerCursor--
		}
	case " ":
		// Toggle multi-selection
		if len(m.providers) > 0 {
			p := m.providers[m.providerCursor]
			m.selectedProviders[p.ID()] = !m.selectedProviders[p.ID()]
		}
	case "enter":
		if len(m.providers) == 0 {
			return m, nil
		}

		// Count selected
		var selected []provider.Provider
		for _, p := range m.providers {
			if m.selectedProviders[p.ID()] {
				selected = append(selected, p)
			}
		}

		if len(selected) > 1 {
			// Multi-provider: go directly to unified search
			m.activeProvider = nil
			m.screen = screenSearch
			m.textInput.Focus()
			return m, nil
		}

		// Single provider: open tabbed view
		m.activeProvider = m.providers[m.providerCursor]
		m.screen = screenSearch
		m.activeTab = tabSearch
		m.textInput.Focus()
		return m, nil
	}
	return m, nil
}

func (m *Model) updateSearchBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If search input is focused:
	if m.textInput.Focused() {
		switch msg.String() {
		case "tab":
			if m.activeProvider != nil && m.activeProvider.Capabilities().CanBrowse {
				m.textInput.Blur()
				m.activeTab = tabBrowse
				m.screen = screenBrowse
				m.isLoading = true
				return m, m.performBrowseCmd()
			}
		case "enter":
			query := m.textInput.Value()
			if query != "" {
				m.textInput.Blur()
				m.isLoading = true
				return m, m.performSearchCmd(query)
			}
		case "down", "ctrl+n", "j":
			m.textInput.Blur()
			if len(m.searchResults) > 0 {
				m.searchCursor = 0
			}
			return m, nil
		case "esc":
			m.textInput.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "tab":
		if m.activeProvider != nil && m.activeProvider.Capabilities().CanBrowse {
			if m.activeTab == tabSearch {
				m.activeTab = tabBrowse
				m.screen = screenBrowse
				m.isLoading = true
				return m, m.performBrowseCmd()
			}
			m.activeTab = tabSearch
			m.screen = screenSearch
			m.textInput.Focus()
			return m, nil
		}

	case "/":
		m.activeTab = tabSearch
		m.screen = screenSearch
		m.textInput.Focus()
		return m, nil

	case "h", "left", "1":
		if m.activeProvider != nil && m.activeTab == tabBrowse {
			m.activeTab = tabSearch
			m.screen = screenSearch
			m.textInput.Focus()
			return m, nil
		}
	case "l", "right", "2":
		if m.activeProvider != nil && m.activeProvider.Capabilities().CanBrowse && m.activeTab == tabSearch {
			m.activeTab = tabBrowse
			m.screen = screenBrowse
			m.isLoading = true
			return m, m.performBrowseCmd()
		}
	case "s":
		// Toggle Popular / Recent sort in Browse tab
		if m.activeTab == tabBrowse && m.activeProvider != nil {
			if m.browseSort == domain.SortPopular {
				m.browseSort = domain.SortRecent
			} else {
				m.browseSort = domain.SortPopular
			}
			m.isLoading = true
			return m, m.performBrowseCmd()
		}

	case "j", "J", "down", "ctrl+n":
		if m.searchCursor < len(m.searchResults)-1 {
			m.searchCursor++
		}
	case "k", "K", "up", "ctrl+p":
		if m.searchCursor > 0 {
			m.searchCursor--
		}
	case "g", "home":
		m.searchCursor = 0
	case "G", "end":
		if len(m.searchResults) > 0 {
			m.searchCursor = len(m.searchResults) - 1
		}
	case "ctrl+d":
		if len(m.searchResults) > 0 {
			m.searchCursor = min(m.searchCursor+8, len(m.searchResults)-1)
		}
	case "ctrl+u":
		m.searchCursor = max(0, m.searchCursor-8)
	case "ctrl+f", "pgdown":
		if len(m.searchResults) > 0 {
			m.searchCursor = min(m.searchCursor+14, len(m.searchResults)-1)
		}
	case "ctrl+b", "pgup":
		m.searchCursor = max(0, m.searchCursor-14)
	case "enter":
		if len(m.searchResults) > 0 {
			selected := m.searchResults[m.searchCursor]
			if m.selectMode {
				m.selectedURL = selected.Series.URL
				m.selectedSeries = &selected.Series
				if selected.Provider != nil {
					m.selectedProvider = selected.Provider.Name()
				}
				if m.searchCancel != nil {
					m.searchCancel()
				}
				if m.cancelFunc != nil {
					m.cancelFunc()
				}
				return m, tea.Quit
			}
			m.activeSeries = selected.Series
			m.activeProvider = selected.Provider
			m.screen = screenChapters
			m.isLoading = true
			return m, m.fetchChaptersCmd(selected.Provider, selected.Series)
		}
	}
	return m, nil
}

func (m *Model) updateChapters(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "J", "down", "ctrl+n":
		if m.chapterCursor < len(m.chapters)-1 {
			m.chapterCursor++
		}
	case "k", "K", "up", "ctrl+p":
		if m.chapterCursor > 0 {
			m.chapterCursor--
		}
	case "g", "home":
		m.chapterCursor = 0
	case "G", "end":
		if len(m.chapters) > 0 {
			m.chapterCursor = len(m.chapters) - 1
		}
	case "ctrl+d":
		if len(m.chapters) > 0 {
			m.chapterCursor = min(m.chapterCursor+8, len(m.chapters)-1)
		}
	case "ctrl+u":
		m.chapterCursor = max(0, m.chapterCursor-8)
	case "ctrl+f", "pgdown":
		if len(m.chapters) > 0 {
			m.chapterCursor = min(m.chapterCursor+14, len(m.chapters)-1)
		}
	case "ctrl+b", "pgup":
		m.chapterCursor = max(0, m.chapterCursor-14)
	case " ":
		// Toggle chapter selection for batch download
		if len(m.chapters) > 0 {
			ch := m.chapters[m.chapterCursor]
			key := chapterKey(ch)
			m.selectedChapters[key] = !m.selectedChapters[key]
		}
	case "a", "tab":
		// Select / Deselect all chapters
		allSelected := len(m.selectedChapters) == len(m.chapters)
		m.selectedChapters = make(map[string]bool)
		if !allSelected {
			for _, ch := range m.chapters {
				m.selectedChapters[chapterKey(ch)] = true
			}
		}
	case "d", "enter":
		return m.queueSelectedChapters()
	}
	return m, nil
}

func (m *Model) updateQueue(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visible := m.filteredQueue()

	switch msg.String() {
	case "j", "J", "down", "ctrl+n":
		if m.queueCursor < len(visible)-1 {
			m.queueCursor++
		}
	case "k", "K", "up", "ctrl+p":
		if m.queueCursor > 0 {
			m.queueCursor--
		}
	case "g", "home":
		m.queueCursor = 0
	case "G", "end":
		if len(visible) > 0 {
			m.queueCursor = len(visible) - 1
		}
	case "ctrl+d":
		if len(visible) > 0 {
			m.queueCursor = min(m.queueCursor+8, len(visible)-1)
		}
	case "ctrl+u":
		m.queueCursor = max(0, m.queueCursor-8)
	case "ctrl+f", "pgdown":
		if len(visible) > 0 {
			m.queueCursor = min(m.queueCursor+14, len(visible)-1)
		}
	case "ctrl+b", "pgup":
		m.queueCursor = max(0, m.queueCursor-14)
	case "f":
		m.hideCompleted = !m.hideCompleted
		if m.queueCursor >= len(m.filteredQueue()) {
			m.queueCursor = 0
		}
	case "c":
		// Clear completed
		var remaining []*QueueItem
		for _, item := range m.queue {
			if item.Status != StatusCompleted {
				remaining = append(remaining, item)
			}
		}
		m.queue = remaining
		m.queueCursor = 0
		m.statusMsg = "Cleared completed downloads"
		return m, dumpQueueAsyncCmd(m.queue)
	case "r":
		// Retry / resume pending or failed downloads
		resumed := 0
		for _, item := range m.queue {
			if item.Status == StatusFailed {
				item.Status = StatusQueued
				item.ErrorMessage = ""
				resumed++
			}
		}
		m.statusMsg = fmt.Sprintf("Resumed %d download(s)", resumed)
		if !m.isDownloading {
			return m.startNextDownload()
		}
	}
	return m, nil
}

func (m *Model) startNextDownload() (tea.Model, tea.Cmd) {
	for _, item := range m.queue {
		if item.Status == StatusQueued {
			item.Status = StatusDownloading
			m.isDownloading = true
			targetItem := item
			return m, func() tea.Msg {
				_, err := m.downloader.DownloadChapter(
					m.ctx,
					targetItem.Provider,
					targetItem.Series,
					targetItem.Chapter,
					downloader.DownloadOptions{},
				)
				return queueDownloadFinishedMsg{
					itemID: targetItem.ID,
					err:    err,
				}
			}
		}
	}
	m.isDownloading = false
	return m, nil
}

func (m *Model) performSearchCmd(query string) tea.Cmd {
	if m.searchCancel != nil {
		m.searchCancel()
	}

	searchCtx, cancel := context.WithCancel(m.ctx)
	m.searchCancel = cancel

	var targetProviders []provider.Provider
	if m.activeProvider != nil {
		targetProviders = []provider.Provider{m.activeProvider}
	} else {
		for _, p := range m.providers {
			if m.selectedProviders[p.ID()] {
				targetProviders = append(targetProviders, p)
			}
		}
	}

	m.searchResults = nil
	m.searchCursor = 0
	m.pendingSearches = len(targetProviders)
	m.isLoading = len(targetProviders) > 0

	var cmds []tea.Cmd
	if m.isLoading {
		cmds = append(cmds, m.spinner.Tick)
	}

	for _, p := range targetProviders {
		prov := p
		cmds = append(cmds, func() tea.Msg {
			pCtx, pCancel := context.WithTimeout(searchCtx, 6*time.Second)
			defer pCancel()

			seriesList, err := prov.Search(pCtx, query)
			return providerSearchChunkMsg{
				provider: prov,
				results:  seriesList,
				err:      err,
			}
		})
	}

	return tea.Batch(cmds...)
}

func (m *Model) performBrowseCmd() tea.Cmd {
	return func() tea.Msg {
		if m.activeProvider == nil {
			return searchResultMsg{}
		}

		seriesList, err := m.activeProvider.Browse(m.ctx, provider.BrowseOptions{
			Sort: m.browseSort,
			Page: 1,
		})
		if err != nil {
			return searchResultMsg{err: err}
		}

		var results []searchResultItem
		for _, s := range seriesList {
			results = append(results, searchResultItem{
				Provider: m.activeProvider,
				Series:   s,
			})
		}
		return searchResultMsg{results: results}
	}
}

func (m *Model) fetchChaptersCmd(p provider.Provider, s domain.Series) tea.Cmd {
	cacheKey := fmt.Sprintf("%s:%s", p.ID(), s.ID)
	if cached, ok := m.chapterCache[cacheKey]; ok && len(cached) > 0 {
		return func() tea.Msg {
			return chaptersResultMsg{chapters: cached, cacheKey: cacheKey}
		}
	}

	return func() tea.Msg {
		chapters, er := p.GetChapters(m.ctx, s)
		return chaptersResultMsg{chapters: chapters, err: er, cacheKey: cacheKey}
	}
}
