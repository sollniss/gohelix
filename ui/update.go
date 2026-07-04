package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sollniss/gohelix/difft"
	"github.com/sollniss/gohelix/runner"
)

func (m Model) Init() tea.Cmd {
	if m.diffMode == DiffDifft {
		// A persisted difft preference needs its first alignment computed up
		// front; route through Update so it runs against the real model (see
		// requestDiffMsg).
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
		// difft output is width-independent.
		m.clampMenuScroll()
		m.clampPreviewScroll()
		m.clampResultScroll()
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
		m.previewScroll = scroll{}
		m.errMsg = ""
		cmd = m.requestDiff()
	case "s":
		m.cycleSplit()
	case "esc":
		m.state = statePreview
	}
	return m, cmd
}

// hScrollStep is how many cells one h/l press shifts the diff columns.
const hScrollStep = 4

// scrollKeys applies the j/k/h/l (and arrow-key) scrolling bindings shared by
// the preview and result views, bounding the offsets to [0, maxY] and
// [0, maxX]. Non-scrolling keys are ignored.
func scrollKeys(msg tea.KeyMsg, sc *scroll, maxY, maxX int) {
	switch msg.String() {
	case "j", "down":
		sc.y = min(sc.y+1, maxY)
	case "k", "up":
		sc.y = max(sc.y-1, 0)
	case "l", "right":
		sc.x = min(sc.x+hScrollStep, maxX)
	case "h", "left":
		sc.x = max(sc.x-hScrollStep, 0)
	}
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
	case "s":
		// Cycle the solution split position for upcoming runs (persisted).
		m.cycleSplit()
	case "?":
		m.showHint = !m.showHint
		m.clampPreviewScroll()
	case "d":
		cmd = m.toggleScroll(&m.previewScroll.y, m.clampPreviewScroll)
	default:
		scrollKeys(msg, &m.previewScroll, m.maxPreviewScroll(), m.maxPreviewHScroll())
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
	case "d":
		cmd = m.toggleScroll(&m.resultScroll.y, m.clampResultScroll)
	default:
		scrollKeys(msg, &m.resultScroll, m.maxResultScroll(), m.maxResultHScroll())
	}
	return m, cmd
}

// toggleScroll switches the diff mode while keeping the left source line at the
// top of the viewport pinned in place, instead of jumping back to the top. y
// points at the active view's vertical offset and clamp bounds it afterwards.
//
// difft→builtin resolves immediately (the builtin layout is positional). builtin→
// difft can't: the alignment is computed asynchronously, so the anchor is stored
// against the new request's seq and applied in applyPendingScroll when it lands.
func (m *Model) toggleScroll(y *int, clamp func()) tea.Cmd {
	// At the very top the diff header (labels + rule) is visible and identical in
	// both modes, so the offset needs no translation.
	if *y < diffHeaderRows {
		return m.toggleDiffMode()
	}
	anchor := scrollToLeftLine(m.diffMode, m.difftResult.Rows, *y)
	cmd := m.toggleDiffMode()
	if m.diffMode == DiffDifft {
		m.pendingScrollLine = anchor
		m.pendingScrollSeq = m.diffSeq
	} else {
		*y = leftLineToScroll(m.diffMode, m.difftResult.Rows, anchor)
		clamp()
	}
	return cmd
}

// applyPendingScroll pins the anchored left source line to the top of the active
// view once a difft alignment has arrived.
func (m *Model) applyPendingScroll() {
	y := leftLineToScroll(m.diffMode, m.difftResult.Rows, m.pendingScrollLine)
	switch m.state {
	case statePreview:
		m.previewScroll.y = y
		m.clampPreviewScroll()
	case stateResult:
		m.resultScroll.y = y
		m.clampResultScroll()
	}
}

// startChallenge runs helix on the current challenge. When the solution split
// is enabled the solution opens read-only at the configured position.
func (m Model) startChallenge() (tea.Model, tea.Cmd) {
	ch := m.current()
	var r *runner.Runner
	if m.splitPos().Enabled() {
		r = runner.NewWithSolution(ch.Before, ch.After, m.splitOptions())
	} else {
		r = runner.New(ch.Before)
	}
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
	m.resultScroll = scroll{}
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
