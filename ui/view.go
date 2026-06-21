package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/sollniss/gohelix/diffview"
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

	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

const (
	menuTitle = "ᛝ  Helix Practice: Challenge Picker"
	menuHelp  = "j/k navigate • enter select • esc back • q quit"
)

func (m Model) menuView() string {
	header := titleStyle.Render(menuTitle) + "\n\n"
	footer := helpStyle.Render(menuHelp)

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

// menuVisibleRows is the number of challenge rows that fit between the sticky
// header and footer.
func (m Model) menuVisibleRows() int {
	header := titleStyle.Render(menuTitle) + "\n\n"
	headerLines := strings.Count(header, "\n")
	footerLines := 2 // margin + footer line
	return max(m.height-headerLines-footerLines-2, 1)
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

func (m Model) previewView() string {
	ch := m.current()

	// sticky footer
	diffToggle := ""
	if m.difftAvailable {
		diffToggle = " • d toggle diff"
	}
	footer := helpStyle.Render("j/k/h/l scroll" + diffToggle + " • enter start • ? hint • esc menu • q quit")

	// code body
	var bodyStr string
	r := diffview.Renderer{Width: m.width}
	if m.diffMode == DiffDifft {
		bodyStr = r.Difft("Before:", "After:", ch.Before, ch.After, m.previewHScroll, m.difftResult)
	} else {
		bodyStr = r.Builtin("Before:", "After:", ch.Before, ch.After, m.previewHScroll)
	}

	// Compose with scrolling
	headerStr := m.previewHeader()

	headerLines := strings.Count(headerStr, "\n")
	footerLines := 2 // margin + footer line

	// Available height for the code body (account for outer padding: 1 top + 1 bottom)
	available := max(m.height-headerLines-footerLines-2, 3)

	bodyLines := strings.Split(bodyStr, "\n")

	// Clamp scroll offset
	maxScroll := max(len(bodyLines)-available, 0)
	scroll := min(m.previewScroll, maxScroll)

	// Slice the visible portion
	end := min(scroll+available, len(bodyLines))
	visibleBody := strings.Join(bodyLines[scroll:end], "\n")

	// Scroll indicator
	scrollHint := ""
	if maxScroll > 0 {
		scrollHint = dimStyle.Render(fmt.Sprintf(" [%d/%d]", scroll, maxScroll))
	}

	var result strings.Builder
	result.WriteString(headerStr)
	result.WriteString(visibleBody)
	result.WriteString("\n") // helpStyle.MarginTop(1) supplies the blank line above the footer
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
		b.WriteString("\n\n")
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

	b.WriteString("\n")
	footer := "r retry • enter next • esc pick challenge • q quit"
	if !m.passed {
		diffToggle := ""
		if m.difftAvailable {
			diffToggle = " • d toggle diff"
		}
		footer = "j/k/h/l scroll" + diffToggle + " • r retry • enter next • esc menu • q quit"
	}
	b.WriteString(helpStyle.Render(footer))

	return b.String()
}

// resultDiffView renders a scrollable side-by-side comparison of the user's output vs expected.
func (m Model) resultDiffView() string {
	ch := m.current()

	var bodyStr string
	r := diffview.Renderer{Width: m.width}
	if m.diffMode == DiffDifft {
		bodyStr = r.Difft("Your Solution:", "Expected:", m.lastResult.Output, ch.After, m.resultHScroll, m.difftResult)
	} else {
		bodyStr = r.Builtin("Your Solution:", "Expected:", m.lastResult.Output, ch.After, m.resultHScroll)
	}

	// Apply vertical scroll
	bodyLines := strings.Split(bodyStr, "\n")

	available := max(m.height/2, 5)
	maxScroll := max(len(bodyLines)-available, 0)
	scroll := min(m.resultScroll, maxScroll)
	end := min(scroll+available, len(bodyLines))

	return strings.Join(bodyLines[scroll:end], "\n")
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

// maxPreviewScroll computes the maximum scroll offset for the current challenge.
func (m Model) maxPreviewScroll() int {
	ch := m.current()

	headerLines := strings.Count(m.previewHeader(), "\n")
	totalBody := m.diffBodyLines(ch.Before, ch.After)

	footerHeight := 2                                        // blank line + help text
	available := max(m.height-headerLines-footerHeight-2, 3) // outer padding

	return max(totalBody-available, 0)
}

// clampPreviewScroll ensures previewScroll doesn't exceed the maximum.
func (m *Model) clampPreviewScroll() {
	m.previewScroll = min(m.previewScroll, m.maxPreviewScroll())
}

// clampResultScroll ensures resultScroll doesn't exceed the maximum.
func (m *Model) clampResultScroll() {
	if m.lastResult == nil || m.passed {
		m.resultScroll = 0
		return
	}
	totalBody := m.diffBodyLines(m.lastResult.Output, m.current().After)
	available := max(m.height/2, 5)
	maxScroll := max(totalBody-available, 0)
	m.resultScroll = min(m.resultScroll, maxScroll)
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
