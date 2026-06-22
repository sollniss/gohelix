package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sollniss/gohelix/difft"
	"github.com/sollniss/gohelix/runner"
)

func (m Model) Init() tea.Cmd {
	if m.diffMode == DiffDifft {
		// Compute the initial diff via the real model in Update.
		return func() tea.Msg { return requestDiffMsg{} }
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// difft output is width-independent (gohelix lays it out and truncates at
		// render time), so a resize needs no recompute.
		m.clampMenuScroll() // the menu window depends on height
		return m, nil
	case requestDiffMsg:
		return m, m.requestDiff()
	case diffReadyMsg:
		if msg.seq == m.diffSeq {
			if msg.aligned != nil {
				m.difftResult = *msg.aligned
			} else {
				m.difftResult = difft.Result{} // difft failed -> fall back to bat view
			}
			// A toggle into difft deferred its scroll anchor until the alignment
			// was ready; now that this exact request's result is in, apply it.
			if m.pendingScrollSeq == msg.seq {
				m.applyPendingScroll()
				m.pendingScrollSeq = 0
			}
		}
		return m, nil
	case challengeCompleteMsg:
		return m.handleComplete(msg.result)
	case challengeErrMsg:
		m.state = stateResult
		m.errMsg = msg.err.Error()
		m.passed = false
		m.lastResult = nil
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.state {
	case stateMenu:
		return m.menuKeys(msg)
	case statePreview:
		return m.previewKeys(msg)
	case stateResult:
		return m.resultKeys(msg)
	}
	return m, nil
}

// openMenu enters the challenge picker scrolled to the top, with the first
// challenge selected.
func (m *Model) openMenu() {
	m.state = stateMenu
	m.menuCursor = 0
	m.menuScroll = 0
}

// clampMenuScroll keeps the selected challenge within the visible window and the
// scroll offset within bounds.
func (m *Model) clampMenuScroll() {
	visible := m.menuVisibleRows()
	if m.menuCursor < m.menuScroll {
		m.menuScroll = m.menuCursor
	} else if m.menuCursor >= m.menuScroll+visible {
		m.menuScroll = m.menuCursor - visible + 1
	}
	maxScroll := max(len(m.challenges)-visible, 0)
	m.menuScroll = max(0, min(m.menuScroll, maxScroll))
}

func (m Model) menuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "j", "down":
		if m.menuCursor < len(m.challenges)-1 {
			m.menuCursor++
		}
		m.clampMenuScroll()
	case "k", "up":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
		m.clampMenuScroll()
	case "enter":
		// Reset the permutation to start at the selected challenge. Overwriting a
		// single slot would duplicate the pick (and drop another challenge), so the
		// same one could reappear on the next advance; a fresh permutation can't.
		m.perm = permStartingAt(len(m.challenges), m.menuCursor)
		m.permIdx = 0
		m.state = statePreview
		m.showHint = false
		m.previewScroll = 0
		m.previewHScroll = 0
		m.errMsg = ""
		cmd = m.requestDiff()
	case "esc":
		m.state = statePreview
	}
	return m, cmd
}

func (m Model) previewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		m.openMenu()
	case "enter":
		return m.startChallenge()
	case "?":
		m.showHint = !m.showHint
		m.clampPreviewScroll()
	case "d":
		cmd = m.toggleScroll(&m.previewScroll, m.clampPreviewScroll)
	case "j", "down":
		m.previewScroll++
		m.clampPreviewScroll()
	case "k", "up":
		if m.previewScroll > 0 {
			m.previewScroll--
		}
	case "l", "right":
		m.previewHScroll += 4
	case "h", "left":
		m.previewHScroll -= 4
		if m.previewHScroll < 0 {
			m.previewHScroll = 0
		}
	}
	return m, cmd
}

func (m Model) resultKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		m.openMenu()
	case "r":
		// Retry same challenge: the preview diffs before vs after.
		m.state = statePreview
		cmd = m.requestDiff()
	case "enter":
		// Next challenge in permutation
		m.advance()
		m.state = statePreview
		cmd = m.requestDiff()
	case "j", "down":
		m.resultScroll++
		m.clampResultScroll()
	case "k", "up":
		if m.resultScroll > 0 {
			m.resultScroll--
		}
	case "l", "right":
		m.resultHScroll += 4
	case "h", "left":
		m.resultHScroll -= 4
		if m.resultHScroll < 0 {
			m.resultHScroll = 0
		}
	case "d":
		cmd = m.toggleScroll(&m.resultScroll, m.clampResultScroll)
	}
	return m, cmd
}

// toggleScroll switches the diff mode while keeping the left source line at the
// top of the viewport pinned in place, instead of jumping back to the top. scroll
// points at the active view's vertical offset and clamp bounds it afterwards.
//
// difft→builtin resolves immediately (the builtin layout is positional). builtin→
// difft can't: the alignment is computed asynchronously, so the anchor is stored
// against the new request's seq and applied in applyPendingScroll when it lands.
func (m *Model) toggleScroll(scroll *int, clamp func()) tea.Cmd {
	// At the very top the diff header (labels + rule) is visible and identical in
	// both modes, so the offset needs no translation.
	if *scroll < diffHeaderRows {
		return m.toggleDiffMode()
	}
	anchor := scrollToLeftLine(m.diffMode, m.difftResult.Rows, *scroll)
	cmd := m.toggleDiffMode()
	if m.diffMode == DiffDifft {
		m.pendingScrollLine = anchor
		m.pendingScrollSeq = m.diffSeq
	} else {
		*scroll = leftLineToScroll(m.diffMode, m.difftResult.Rows, anchor)
		clamp()
	}
	return cmd
}

// applyPendingScroll pins the anchored left source line to the top of the active
// view once a difft alignment has arrived.
func (m *Model) applyPendingScroll() {
	scroll := leftLineToScroll(m.diffMode, m.difftResult.Rows, m.pendingScrollLine)
	switch m.state {
	case statePreview:
		m.previewScroll = scroll
		m.clampPreviewScroll()
	case stateResult:
		m.resultScroll = scroll
		m.clampResultScroll()
	}
}

func (m Model) startChallenge() (tea.Model, tea.Cmd) {
	ch := m.current()
	r := runner.New(ch.Before)
	return m, tea.Exec(r, func(err error) tea.Msg {
		if err != nil {
			return challengeErrMsg{err}
		}
		return challengeCompleteMsg{r.GetResult()}
	})
}

func (m Model) handleComplete(result runner.Result) (tea.Model, tea.Cmd) {
	m.state = stateResult
	m.lastResult = &result
	m.errMsg = ""
	m.resultScroll = 0
	m.resultHScroll = 0
	ch := m.current()
	m.passed = result.Output == ch.After

	var cmd tea.Cmd
	if m.passed {
		m.isRecord = m.scores.Update(ch.ID, result.Keystrokes, result.Duration)
	} else {
		m.isRecord = false
		cmd = m.requestDiff() // diff the failed output against the expected result
	}
	return m, cmd
}
