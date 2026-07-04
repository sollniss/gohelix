package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/sollniss/gohelix/diffview"
	"github.com/sollniss/gohelix/runner"
)

// View renders the current state.
func (m Model) View() string {
	var content string
	switch m.state {
	case stateMenu:
		content = m.menuView()
	case statePreview:
		content = m.previewView()
	case stateResult:
		content = m.resultView()
	default:
		content = ""
	}

	rendered := lipgloss.NewStyle().Padding(1, 2).Render(content)

	if m.height > 0 {
		if lines := strings.Split(rendered, "\n"); len(lines) > m.height {
			rendered = strings.Join(lines[:m.height], "\n")
		}
	}
	return rendered
}

const menuTitle = "ᛝ  Helix Practice: Challenge Picker"

func (m Model) menuView() string {
	header := titleStyle.Render(menuTitle) + "\n\n"
	footer := helpStyle.Render(fmt.Sprintf("j/k navigate • enter select • s split: %s • esc back • q quit", m.splitPos()))

	// Build every challenge row, then show only the window that fits the height.
	lines := make([]string, len(m.challenges))
	for i, ch := range m.challenges {
		cursor := "  "
		style := normalStyle
		if i == m.menuCursor {
			cursor = "❯ "
			style = selectedStyle
		}

		num := fmt.Sprintf("%2d. %s", i+1, ch.Title)
		line := style.Render(cursor + num)

		// Score display
		if entry, ok := m.scores.Get(ch.ID); ok {
			sc := fmt.Sprintf("  ⚡ %d  ⏱  %s", entry.Keystrokes, formatDuration(entry.Duration))
			line += scoreStyle.Render(sc)
		}
		lines[i] = line
	}

	// Clamp the window to a valid range (menuScroll may be stale after a resize).
	visible := m.menuVisibleRows()
	start := max(0, min(m.menuScroll, max(len(lines)-visible, 0)))
	end := min(start+visible, len(lines))

	var b strings.Builder
	b.WriteString(header)
	for i := start; i < end; i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
	}
	// Pad the list to a fixed height so the footer stays pinned to the bottom.
	for i := end - start; i < visible; i++ {
		b.WriteString("\n")
	}
	b.WriteString(footer)

	return b.String()
}

// Layout accounting.
const (
	// footerRows is the help line plus the blank margin helpStyle.MarginTop(1)
	// renders above it.
	footerRows = 2
	// outerPadRows is View's Padding(1, 2): one row above, one below.
	outerPadRows = 2
)

// bodyRows returns the rows available to a scrollable body, given the chrome
// rows around it (everything between the outer padding and the footer that
// isn't body). floor keeps a few body rows visible on tiny terminals; View's
// height clamp keeps the top of the screen visible then.
func (m Model) bodyRows(chrome, floor int) int {
	return max(m.height-chrome-footerRows-outerPadRows, floor)
}

// menuVisibleRows is the number of challenge rows that fit between the sticky
// header and footer.
func (m Model) menuVisibleRows() int {
	header := titleStyle.Render(menuTitle) + "\n\n"
	return m.bodyRows(strings.Count(header, "\n"), 1)
}

// previewHeader renders the title/hint/score block shown above the diff body.
func (m Model) previewHeader() string {
	ch := m.current()

	var header strings.Builder
	header.WriteString(titleStyle.Render(ch.Title))
	header.WriteString("\n")

	if m.showHint {
		header.WriteString(hintStyle.Render("💡 " + ch.Hint))
		header.WriteString("\n")
	}

	if entry, ok := m.scores.Get(ch.ID); ok {
		header.WriteString(scoreStyle.Render(fmt.Sprintf("Best: ⚡ %d  ⏱  %s", entry.Keystrokes, formatDuration(entry.Duration))))
		header.WriteString("\n\n")
	}

	return header.String()
}

// previewFooter builds the preview help line. The peek key is announced next
// to the split setting, but only when the split is enabled.
func (m Model) previewFooter() string {
	diffToggle := ""
	if m.difftAvailable {
		diffToggle = " • d diff"
	}
	peek := ""
	if m.splitPos().Enabled() {
		peek = fmt.Sprintf(" • %s peek", runner.PeekKeyName(m.peekKey()))
	}
	return fmt.Sprintf("j/k/h/l scroll%s • enter start • s split: %s%s • ? hint • esc menu • q quit", diffToggle, m.splitPos(), peek)
}

func (m Model) previewView() string {
	ch := m.current()

	// sticky footer
	footer := helpStyle.Render(m.previewFooter())

	// code body
	var bodyStr string
	r := diffview.Renderer{Width: m.width}
	if m.diffMode == DiffDifft {
		bodyStr = r.Difft("Before:", "After:", ch.Before, ch.After, m.previewScroll.x, m.difftResult)
	} else {
		bodyStr = r.Builtin("Before:", "After:", ch.Before, ch.After, m.previewScroll.x)
	}

	// Compose with scrolling. The window and its limit come from the same
	// accounting the key handlers clamp with (previewBodyRows/maxPreviewScroll),
	// so the indicator, the clamp and the rendered slice can't disagree.
	headerStr := m.previewHeader()
	available := m.previewBodyRows()
	maxScroll := m.maxPreviewScroll()

	bodyLines := strings.Split(bodyStr, "\n")
	top := min(m.previewScroll.y, maxScroll, len(bodyLines))
	end := min(top+available, len(bodyLines))
	visibleBody := strings.Join(bodyLines[top:end], "\n")

	// Scroll indicator
	scrollHint := ""
	if maxScroll > 0 {
		scrollHint = dimStyle.Render(fmt.Sprintf(" [%d/%d]", top, maxScroll))
	}

	var result strings.Builder
	result.WriteString(headerStr)
	result.WriteString(visibleBody)
	result.WriteString("\n")
	result.WriteString(footer)
	result.WriteString(scrollHint)

	return result.String()
}

func (m Model) resultView() string {
	var b strings.Builder

	if m.errMsg != "" {
		b.WriteString(failStyle.Render("✗ Error"))
		b.WriteString("\n\n")
		b.WriteString(normalStyle.Render(m.errMsg))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("r retry • esc pick challenge • q quit"))
		return b.String()
	}

	if m.passed {
		b.WriteString(passStyle.Render("✓ Challenge Passed!"))
	} else {
		b.WriteString(failStyle.Render("✗ Not quite right"))
	}
	b.WriteString("\n\n")

	if m.lastResult != nil {
		b.WriteString(normalStyle.Render(fmt.Sprintf("  ⚡ Keystrokes:  %d", m.lastResult.Keystrokes)))
		b.WriteString("\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("  ⏱  Time:        %s", formatDuration(m.lastResult.Duration))))
		b.WriteString("\n")
	}

	if m.isRecord {
		b.WriteString("\n")
		b.WriteString(recordStyle.Render("  🏆 New Record!"))
		b.WriteString("\n")
	}

	// Show diff when failed
	if !m.passed && m.lastResult != nil {
		b.WriteString("\n")
		b.WriteString(m.resultDiffView())
		b.WriteString("\n")
	}

	// No explicit blank here: helpStyle.MarginTop(1) supplies the one blank
	// line above the footer, same as the preview and menu.
	footer := "r retry • enter next • esc pick challenge • q quit"
	if !m.passed {
		diffToggle := ""
		if m.difftAvailable {
			diffToggle = " • d diff"
		}
		footer = "j/k/h/l scroll" + diffToggle + " • r retry • enter next • esc menu • q quit"
	}
	b.WriteString(helpStyle.Render(footer))

	return b.String()
}

// resultChromeRows is what surrounds the result diff besides the footer and
// padding: the status line, a blank, the two stat lines and a blank above the
// diff.
const resultChromeRows = 5

// resultBodyRows is the diff window height in the result view. Sizing the diff
// to fit keeps the whole view within the terminal.
func (m Model) resultBodyRows() int {
	return m.bodyRows(resultChromeRows, 5)
}

// previewBodyRows is the diff window height in the preview: what is left after
// the header block (title, optional hint, optional best score).
func (m Model) previewBodyRows() int {
	return m.bodyRows(strings.Count(m.previewHeader(), "\n"), 3)
}

// resultDiffView renders a scrollable side-by-side comparison of the user's output vs expected.
func (m Model) resultDiffView() string {
	ch := m.current()

	var bodyStr string
	r := diffview.Renderer{Width: m.width}
	if m.diffMode == DiffDifft {
		bodyStr = r.Difft("Your Solution:", "Expected:", m.lastResult.Output, ch.After, m.resultScroll.x, m.difftResult)
	} else {
		bodyStr = r.Builtin("Your Solution:", "Expected:", m.lastResult.Output, ch.After, m.resultScroll.x)
	}

	// Same accounting as the key handlers' clamp, so the visible window and
	// the scroll limits agree.
	bodyLines := strings.Split(bodyStr, "\n")
	top := min(m.resultScroll.y, m.maxResultScroll(), len(bodyLines))
	end := min(top+m.resultBodyRows(), len(bodyLines))

	return strings.Join(bodyLines[top:end], "\n")
}

// diffBodyLines reports how many lines the diff body renders for the given
// left/right sources.
func (m Model) diffBodyLines(left, right string) int {
	if m.diffMode == DiffDifft && len(m.difftResult.Rows) > 0 {
		return diffHeaderRows + len(m.difftResult.Rows) + 1
	}
	leftLines := strings.Count(strings.TrimRight(left, "\n"), "\n") + 1
	rightLines := strings.Count(strings.TrimRight(right, "\n"), "\n") + 1
	return diffHeaderRows + max(leftLines, rightLines) + 1
}

// maxPreviewScroll is the largest top offset for the preview diff.
func (m Model) maxPreviewScroll() int {
	ch := m.current()
	return max(m.diffBodyLines(ch.Before, ch.After)-m.previewBodyRows(), 0)
}

// maxResultScroll is the largest top offset for the result diff: zero when
// there is nothing to scroll (no result yet, or a passed run shows no diff).
func (m Model) maxResultScroll() int {
	if m.lastResult == nil || m.passed {
		return 0
	}
	return max(m.diffBodyLines(m.lastResult.Output, m.current().After)-m.resultBodyRows(), 0)
}

// maxPreviewHScroll bounds horizontal scrolling to the widest preview line.
func (m Model) maxPreviewHScroll() int {
	ch := m.current()
	return diffview.Renderer{Width: m.width}.MaxHScroll(ch.Before, ch.After)
}

// maxResultHScroll bounds horizontal scrolling to the widest result-diff line.
func (m Model) maxResultHScroll() int {
	if m.lastResult == nil || m.passed {
		return 0
	}
	return diffview.Renderer{Width: m.width}.MaxHScroll(m.lastResult.Output, m.current().After)
}

// clampPreviewScroll pulls the preview offsets back into range after a limit
// shrank (hint toggled, diff mode toggled, resize).
func (m *Model) clampPreviewScroll() {
	m.previewScroll.y = min(m.previewScroll.y, m.maxPreviewScroll())
	m.previewScroll.x = min(m.previewScroll.x, m.maxPreviewHScroll())
}

// clampResultScroll is clampPreviewScroll for the result diff.
func (m *Model) clampResultScroll() {
	m.resultScroll.y = min(m.resultScroll.y, m.maxResultScroll())
	m.resultScroll.x = min(m.resultScroll.x, m.maxResultHScroll())
}

// diffHeaderRows is the number of rows (column labels + top rule) the diff body
// places before its first content row; see diffview.renderSideBySide.
const diffHeaderRows = 2

// leftLineAtRow maps a content-row index to the left-column source line it shows.
func leftLineAtRow(mode DiffMode, rows [][2]int, row int) int {
	if row <= 0 {
		return 0
	}
	if mode == DiffDifft && len(rows) > 0 {
		for j := min(row, len(rows)-1); j >= 0; j-- {
			if rows[j][0] >= 0 {
				return rows[j][0]
			}
		}
		return 0
	}
	return row
}

// rowForLeftLine is the inverse of leftLineAtRow: the content-row index that
// shows the given left source line.
func rowForLeftLine(mode DiffMode, rows [][2]int, line int) int {
	if mode == DiffDifft && len(rows) > 0 {
		for i, r := range rows {
			if r[0] == line {
				return i
			}
		}
	}
	return line
}

// scrollToLeftLine reports the left source line at the top of the viewport for a
// given scroll offset.
func scrollToLeftLine(mode DiffMode, rows [][2]int, scroll int) int {
	if scroll < diffHeaderRows {
		return 0
	}
	return leftLineAtRow(mode, rows, scroll-diffHeaderRows)
}

// leftLineToScroll returns the scroll offset that puts the given left source line
// at the top of the viewport.
func leftLineToScroll(mode DiffMode, rows [][2]int, line int) int {
	return rowForLeftLine(mode, rows, line) + diffHeaderRows
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.3fs", d.Seconds())
}
