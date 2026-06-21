package diffview

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/sollniss/gohelix/difft"
)

// Styles for column labels and rules.
var (
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("4")). // blue
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")) // bright black (gray)
)

// Renderer lays out a side-by-side diff. Width is the terminal width; when it is
// zero or negative a default column width is used.
type Renderer struct {
	Width int
}

// Builtin renders a syntax-highlighted (via bat) side-by-side comparison of left and right
// content under the given column labels, scrolled horizontally by hOffset.
func (r Renderer) Builtin(leftLabel, rightLabel, left, right string, hOffset int) string {
	leftLines := strings.Split(highlight(strings.TrimRight(left, "\n")), "\n")
	rightLines := strings.Split(highlight(strings.TrimRight(right, "\n")), "\n")
	return r.renderSideBySide(leftLabel, rightLabel, leftLines, rightLines, hOffset)
}

// Difft renders a diff using difft's line pairing and change
// spans. before/after lines are syntax-highlighted with bat (as in Builtin);
// difft decides which lines face each other and which spans changed.
func (r Renderer) Difft(leftLabel, rightLabel, left, right string, hOffset int, aligned difft.Result) string {
	rows := aligned.Rows
	if len(rows) == 0 {
		return r.Builtin(leftLabel, rightLabel, left, right, hOffset)
	}
	leftSrc := strings.Split(strings.TrimRight(left, "\n"), "\n")
	rightSrc := strings.Split(strings.TrimRight(right, "\n"), "\n")
	leftHL := strings.Split(highlight(strings.TrimRight(left, "\n")), "\n")
	rightHL := strings.Split(highlight(strings.TrimRight(right, "\n")), "\n")

	leftCells := make([]string, len(rows))
	rightCells := make([]string, len(rows))
	srcAt := func(lines []string, i int) string {
		if i >= 0 && i < len(lines) {
			return lines[i]
		}
		return ""
	}
	for i, row := range rows {
		if l := row[0]; l >= 0 && l < len(leftHL) {
			leftCells[i] = highlightChanges(leftHL[l], srcAt(leftSrc, l), aligned.Lhs[l], delBg)
		}
		if rr := row[1]; rr >= 0 && rr < len(rightHL) {
			rightCells[i] = highlightChanges(rightHL[rr], srcAt(rightSrc, rr), aligned.Rhs[rr], addBg)
		}
	}
	return r.renderSideBySide(leftLabel, rightLabel, leftCells, rightCells, hOffset)
}

// Background hues for changed spans. Dark shades keep bat's foreground syntax
// colors readable; deletions (left side) are reddish, insertions (right) greenish.
const (
	delBg = "\x1b[48;5;52m" // dark red background
	addBg = "\x1b[48;5;22m" // dark green background
)

// highlightChanges overlays a background (setCode) onto the changed character
// spans of an already-bat-highlighted line, without disturbing its foreground
// colors. ranges are in source-rune offsets of srcLine; they are mapped to the
// highlighted line's visible columns (tabs count as 4, matching bat --tabs=4).
// The background is re-asserted after every bat reset (\x1b[0m) so it survives
// token boundaries.
func highlightChanges(highlighted, srcLine string, ranges []difft.Range, setCode string) string {
	if len(ranges) == 0 {
		return highlighted
	}

	// Map source-rune offsets to visible columns.
	srcRunes := []rune(srcLine)
	colOf := make([]int, len(srcRunes)+1)
	col := 0
	for i, r := range srcRunes {
		colOf[i] = col
		if r == '\t' {
			col += 4
		} else {
			col++
		}
	}
	colOf[len(srcRunes)] = col

	type span struct{ a, b int } // visible-column range [a, b)
	spans := make([]span, 0, len(ranges))
	for _, rg := range ranges {
		s := min(max(rg.Start, 0), len(srcRunes))
		e := min(max(rg.End, 0), len(srcRunes))
		if e > s {
			spans = append(spans, span{colOf[s], colOf[e]})
		}
	}
	inSpan := func(c int) bool {
		for _, sp := range spans {
			if c >= sp.a && c < sp.b {
				return true
			}
		}
		return false
	}

	const clearCode = "\x1b[49m" // reset background to default
	var b strings.Builder
	visCol := 0
	bgOn := false
	i := 0
	for i < len(highlighted) {
		if highlighted[i] == '\x1b' && i+1 < len(highlighted) && highlighted[i+1] == '[' {
			j := i + 2
			for j < len(highlighted) && highlighted[j] != 'm' {
				j++
			}
			if j < len(highlighted) {
				j++
			}
			seq := highlighted[i:j]
			b.WriteString(seq)
			if seq == "\x1b[0m" {
				bgOn = false // bat reset cleared our background too
			}
			i = j
			continue
		}

		want := inSpan(visCol)
		if want && !bgOn {
			b.WriteString(setCode)
			bgOn = true
		} else if !want && bgOn {
			b.WriteString(clearCode)
			bgOn = false
		}

		_, size := utf8.DecodeRuneInString(highlighted[i:])
		b.WriteString(highlighted[i : i+size])
		// Tabs advance to the next 4-column stop (matches bat --tabs=4 and colOf),
		// so the no-bat fallback (which keeps literal tabs) stays aligned.
		if highlighted[i] == '\t' {
			visCol += 4
		} else {
			visCol++
		}
		i += size
	}
	if bgOn {
		b.WriteString(clearCode)
	}
	return b.String()
}

// renderSideBySide lays out pre-split, already-highlighted left/right lines into
// two equal columns (each half the terminal width) with horizontal scrolling.
func (r Renderer) renderSideBySide(
	leftLabel, rightLabel string,
	leftLines, rightLines []string,
	hOffset int,
) string {
	colWidth := 38
	if r.Width > 0 {
		colWidth = max((r.Width-8)/2, 20)
	}
	rule := dimStyle.Render(strings.Repeat("┄", colWidth))

	// columnGap is the spacing between the two diff columns.
	const columnGap = "    "

	var body strings.Builder
	body.WriteString(labelStyle.Render(padRight(leftLabel, colWidth)))
	body.WriteString(columnGap)
	body.WriteString(labelStyle.Render(rightLabel))
	body.WriteString("\n")

	body.WriteString(dimStyle.Render(padRight(strings.Repeat("┄", colWidth), colWidth)))
	body.WriteString(columnGap)
	body.WriteString(rule)
	body.WriteString("\n")

	maxLines := max(len(leftLines), len(rightLines))

	for i := range maxLines {
		l := ""
		if i < len(leftLines) {
			l = ansi.TruncateLeft(leftLines[i], hOffset, "")
		}
		rr := ""
		if i < len(rightLines) {
			rr = ansi.TruncateLeft(rightLines[i], hOffset, "")
		}

		body.WriteString(padRight(l, colWidth))
		body.WriteString(columnGap)
		// The right cell is the last column, so it only needs truncation.
		body.WriteString(ansi.Truncate(rr, colWidth, "…"))
		body.WriteString("\n")
	}

	body.WriteString(dimStyle.Render(padRight(strings.Repeat("┄", colWidth), colWidth)))
	body.WriteString(columnGap)
	body.WriteString(rule)

	return body.String()
}

// bat caches the bat lookup and provides syntax highlighting.
var bat = struct {
	once sync.Once
	path string // empty if bat is not available
}{
	path: "",
}

// highlightCache memoizes highlighted output keyed by source text, so that
// repeated renders (every keystroke) don't re-spawn a bat process per frame.
var highlightCache sync.Map // string -> string

// highlight runs the given Go source through bat for syntax highlighting.
// Returns the ANSI-colored string, or the tab-expanded plain text if bat is
// unavailable. Results are cached, since challenge/result text is stable across
// renders.
func highlight(src string) string {
	if v, ok := highlightCache.Load(src); ok {
		return v.(string)
	}

	bat.once.Do(func() {
		p, err := exec.LookPath("bat")
		if err != nil {
			return
		}
		bat.path = p
	})

	if bat.path == "" {
		return expandTabs(src)
	}

	cmd := exec.Command(
		bat.path,
		"--color=always",
		"--style=plain",
		"--language=go",
		"--paging=never",
		"--tabs="+strconv.Itoa(len(tabSpace)),
	)
	cmd.Stdin = strings.NewReader(src)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return expandTabs(src) // fallback to plain text
	}

	result := strings.TrimRight(out.String(), "\n")
	highlightCache.Store(src, result)
	return result
}

const tabSpace = "    "

// expandTabs replaces tabs with tabSpace so that downstream width math (which
// uses ansi.StringWidth, where a tab counts as zero cells) stays aligned. bat
// already emits spaces via --tabs=4; this only matters for the no-bat fallback.
func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", tabSpace)
}

// padRight truncates s to width cells (appending "…" if it overflows) and
// right-pads with spaces otherwise. Width math is ANSI- and wide-char-aware.
func padRight(s string, width int) string {
	s = ansi.Truncate(s, width, "…")
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
