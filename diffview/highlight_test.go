package diffview

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/sollniss/gohelix/difft"
)

func TestHighlightChanges(t *testing.T) {
	// Plain (no ANSI) line; change span covers runes 4..7 ("bar").
	src := "foo bar baz"
	out := highlightChanges(src, src, []difft.Range{{Start: 4, End: 7}}, addBg)
	if !strings.Contains(out, addBg+"bar") {
		t.Errorf("expected background before 'bar', got %q", out)
	}
	if !strings.Contains(out, "\x1b[49m") {
		t.Errorf("expected background reset after span, got %q", out)
	}

	// Tab expands to 4 columns: a change at rune 1 maps to visible col 4.
	tabSrc := "\tx"
	tabOut := highlightChanges(tabSrc, tabSrc, []difft.Range{{Start: 1, End: 2}}, addBg)
	if !strings.Contains(tabOut, addBg+"x") {
		t.Errorf("expected background before 'x' after tab, got %q", tabOut)
	}

	// No ranges: line returned unchanged.
	if got := highlightChanges(src, src, nil, addBg); got != src {
		t.Errorf("expected unchanged line, got %q", got)
	}
}

func TestPadRight(t *testing.T) {
	// Short string is padded to width.
	if got := padRight("ab", 5); got != "ab   " {
		t.Errorf("padRight(\"ab\", 5) = %q, want %q", got, "ab   ")
	}
	// Exact width is left untouched (no ellipsis).
	if got := padRight("abcde", 5); got != "abcde" {
		t.Errorf("padRight(exact) = %q, want %q", got, "abcde")
	}
	// Overflow is truncated with an ellipsis, staying within width.
	got := padRight("abcdef", 5)
	if w := ansi.StringWidth(got); w != 5 {
		t.Errorf("padRight(overflow) width = %d, want 5 (%q)", w, got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("padRight(overflow) = %q, want trailing ellipsis", got)
	}
	// ANSI escapes don't count toward width: the visible "ab" pads to 5.
	colored := "\x1b[31mab\x1b[0m"
	if w := ansi.StringWidth(padRight(colored, 5)); w != 5 {
		t.Errorf("padRight(colored) width = %d, want 5", w)
	}
}

// strippedRows renders the two columns and removes any ANSI styling so layout
// assertions don't depend on the terminal's detected color profile.
func strippedRows(r Renderer, leftLines, rightLines []string) []string {
	out := r.renderSideBySide("L", "R", leftLines, rightLines, 0)
	return strings.Split(ansi.Strip(out), "\n")
}

func TestRenderSideBySideLayout(t *testing.T) {
	r := Renderer{Width: 48} // colWidth = max((48-8)/2, 20) = 20
	lines := strippedRows(r, []string{"left"}, []string{"right"})

	// label row + rule + 1 content row + closing rule = 4 lines.
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4: %q", len(lines), lines)
	}
	// Label row: left label padded to 20 cells, 4-space gutter, right label.
	if want := "L" + strings.Repeat(" ", 19) + "    " + "R"; lines[0] != want {
		t.Errorf("label row = %q, want %q", lines[0], want)
	}
	// Content row: "left" padded to the 20-cell column before the gutter.
	if want := "left" + strings.Repeat(" ", 16) + "    right"; lines[2] != want {
		t.Errorf("content row = %q, want %q", lines[2], want)
	}
}

func TestRenderSideBySideTruncatesBothColumns(t *testing.T) {
	r := Renderer{Width: 48} // colWidth = 20
	long := strings.Repeat("x", 40)
	lines := strippedRows(r, []string{long}, []string{long})
	content := lines[2]

	// Both cells overflow the 20-cell column, so both end in an ellipsis: the
	// left cell before the gutter and the right cell at the end of the line.
	left, right, ok := strings.Cut(content, "    ")
	if !ok {
		t.Fatalf("no column gutter in content row %q", content)
	}
	if !strings.HasSuffix(left, "…") {
		t.Errorf("left cell not truncated with ellipsis: %q", left)
	}
	if !strings.HasSuffix(right, "…") {
		t.Errorf("right cell not truncated with ellipsis: %q", right)
	}
	// Each cell stays within the column width.
	if w := ansi.StringWidth(left); w != 20 {
		t.Errorf("left cell width = %d, want 20", w)
	}
	if w := ansi.StringWidth(right); w != 20 {
		t.Errorf("right cell width = %d, want 20", w)
	}
}

func TestRenderSideBySideUnevenRows(t *testing.T) {
	// The taller side dictates the row count; the short side pads with blanks.
	r := Renderer{Width: 48}
	lines := strippedRows(r, []string{"a", "b", "c"}, []string{"x"})
	// label row + rule + 3 content + closing rule = 6.
	if len(lines) != 6 {
		t.Fatalf("got %d lines, want 6: %q", len(lines), lines)
	}
}

func TestDifftFallsBackToBuiltinWhenEmpty(t *testing.T) {
	// With no alignment, Difft must produce exactly the Builtin layout.
	var r Renderer
	left, right := "a\nb", "a\nc"
	builtin := r.Builtin("L", "R", left, right, 0)
	fallback := r.Difft("L", "R", left, right, 0, difft.Result{})
	if builtin != fallback {
		t.Errorf("Difft with empty Result should equal Builtin:\nbuiltin=%q\nfallback=%q", builtin, fallback)
	}
}
