package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/omarys/labrador/internal/domain"
)

// handleMouse processes touch taps, clicks, and scroll wheel events from foot and other modern terminals.
func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		switch m.screen {
		case screenProviders:
			if m.providerCursor > 0 {
				m.providerCursor--
			}
		case screenSearch, screenBrowse:
			if m.searchCursor > 0 {
				m.searchCursor--
			}
		case screenChapters:
			if m.chapterCursor > 0 {
				m.chapterCursor--
			}
		case screenQueue:
			if m.queueCursor > 0 {
				m.queueCursor--
			}
		}
		return m, nil

	case tea.MouseButtonWheelDown:
		switch m.screen {
		case screenProviders:
			if m.providerCursor < len(m.providers)-1 {
				m.providerCursor++
			}
		case screenSearch, screenBrowse:
			if m.searchCursor < len(m.searchResults)-1 {
				m.searchCursor++
			}
		case screenChapters:
			if m.chapterCursor < len(m.chapters)-1 {
				m.chapterCursor++
			}
		case screenQueue:
			visible := m.filteredQueue()
			if m.queueCursor < len(visible)-1 {
				m.queueCursor++
			}
		}
		return m, nil

	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}

		now := time.Now()
		isDoubleTap := now.Sub(m.lastClickTime) < 450*time.Millisecond && m.lastClickRow == msg.Y
		m.lastClickTime = now
		m.lastClickRow = msg.Y

		// Check if clicking bottom Touch Action Bar
		if m.height > 0 && msg.Y >= m.height-3 {
			return m.handleTouchBarClick(msg.X)
		}

		// Check screen-specific row taps
		switch m.screen {
		case screenProviders:
			target := msg.Y - 4
			if target >= 0 && target < len(m.providers) {
				m.providerCursor = target
				if isDoubleTap {
					p := m.providers[target]
					m.selectedProviders[p.ID()] = !m.selectedProviders[p.ID()]
				}
			}

		case screenSearch, screenBrowse:
			target := msg.Y - 5
			maxVisible := 14
			if m.height > 10 {
				maxVisible = m.height - 8
			}
			start := 0
			if m.searchCursor >= maxVisible {
				start = m.searchCursor - maxVisible + 1
			}
			idx := start + target
			if target >= 0 && idx < len(m.searchResults) {
				if m.searchCursor == idx && isDoubleTap {
					selected := m.searchResults[idx]
					if m.selectMode {
						m.selectedURL = selected.Series.URL
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
				m.searchCursor = idx
			}

		case screenChapters:
			target := msg.Y - 5
			maxVisible := 16
			if m.height > 10 {
				maxVisible = m.height - 10
			}
			start := 0
			if m.chapterCursor >= maxVisible {
				start = m.chapterCursor - maxVisible + 1
			}
			idx := start + target
			if target >= 0 && idx < len(m.chapters) {
				m.chapterCursor = idx
				ch := m.chapters[idx]
				key := chapterKey(ch)
				m.selectedChapters[key] = !m.selectedChapters[key]
			}

		case screenQueue:
			target := msg.Y - 8
			visible := m.filteredQueue()
			if target >= 0 && target < len(visible) {
				m.queueCursor = target
			}
		}
	}
	return m, nil
}

func (m *Model) handleTouchBarClick(x int) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenSearch, screenBrowse:
		if m.selectMode {
			if x < 15 && len(m.searchResults) > 0 {
				selected := m.searchResults[m.searchCursor]
				m.selectedURL = selected.Series.URL
				if m.searchCancel != nil {
					m.searchCancel()
				}
				if m.cancelFunc != nil {
					m.cancelFunc()
				}
				return m, tea.Quit
			} else if x >= 15 && x < 30 {
				m.textInput.Focus()
				return m, nil
			} else if x >= 30 {
				return m, tea.Quit
			}
		} else {
			if x < 14 && len(m.searchResults) > 0 {
				selected := m.searchResults[m.searchCursor]
				m.activeSeries = selected.Series
				m.activeProvider = selected.Provider
				m.screen = screenChapters
				m.isLoading = true
				return m, m.fetchChaptersCmd(selected.Provider, selected.Series)
			} else if x >= 14 && x < 29 {
				m.textInput.Focus()
				return m, nil
			} else if x >= 29 && x < 45 {
				m.previousScreen = m.screen
				m.screen = screenQueue
				m.queueCursor = 0
				return m, nil
			} else if x >= 45 {
				return m.handleBack()
			}
		}

	case screenChapters:
		if x < 15 && len(m.chapters) > 0 {
			ch := m.chapters[m.chapterCursor]
			key := chapterKey(ch)
			m.selectedChapters[key] = !m.selectedChapters[key]
		} else if x >= 15 && x < 27 {
			allSelected := len(m.selectedChapters) == len(m.chapters)
			m.selectedChapters = make(map[string]bool)
			if !allSelected {
				for _, ch := range m.chapters {
					m.selectedChapters[chapterKey(ch)] = true
				}
			}
		} else if x >= 27 && x < 45 {
			return m.queueSelectedChapters()
		} else if x >= 45 && x < 59 {
			m.previousScreen = m.screen
			m.screen = screenQueue
			m.queueCursor = 0
		} else if x >= 59 {
			return m.handleBack()
		}

	case screenQueue:
		if x < 15 {
			for _, it := range m.queue {
				if it.Status == StatusFailed {
					it.Status = StatusQueued
					it.ErrorMessage = ""
				}
			}
			return m.startNextDownload()
		} else if x >= 15 && x < 31 {
			m.hideCompleted = !m.hideCompleted
		} else if x >= 31 && x < 45 {
			var remaining []*QueueItem
			for _, it := range m.queue {
				if it.Status != StatusCompleted {
					remaining = append(remaining, it)
				}
			}
			m.queue = remaining
			return m, dumpQueueAsyncCmd(m.queue)
		} else if x >= 45 {
			m.screen = m.previousScreen
		}
	}
	return m, nil
}

func (m *Model) queueSelectedChapters() (tea.Model, tea.Cmd) {
	var toQueue []domain.Chapter
	for _, ch := range m.chapters {
		if m.selectedChapters[chapterKey(ch)] {
			toQueue = append(toQueue, ch)
		}
	}
	if len(toQueue) == 0 && len(m.chapters) > 0 {
		toQueue = append(toQueue, m.chapters[m.chapterCursor])
	}

	if len(toQueue) > 0 {
		for _, ch := range toQueue {
			chKey := chapterKey(ch)
			m.queue = append(m.queue, &QueueItem{
				ID:       fmt.Sprintf("%s:%s:%s", m.activeProvider.ID(), m.activeSeries.ID, chKey),
				Provider: m.activeProvider,
				Series:   m.activeSeries,
				Chapter:  ch,
				Status:   StatusQueued,
				AddedAt:  time.Now(),
			})
		}
		m.statusMsg = fmt.Sprintf("Added %d chapter(s) to queue (press 'Q' to view)", len(toQueue))
		m.selectedChapters = make(map[string]bool)

		if !m.isDownloading {
			nextM, nextCmd := m.startNextDownload()
			return nextM, tea.Batch(dumpQueueAsyncCmd(m.queue), nextCmd)
		}
		return m, dumpQueueAsyncCmd(m.queue)
	}
	return m, nil
}
