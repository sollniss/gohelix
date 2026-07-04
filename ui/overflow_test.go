package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/sollniss/gohelix/runner"
)

// tallResultModel builds a failed-result model whose diff is much taller than
// any test terminal, at the given size.
func tallResultModel(t *testing.T, width, height int) Model {
	m := newTestModel(t)
	m.challenges[0].Before = strings.Repeat("var a = 1\n", 40)
	m.challenges[0].After = strings.Repeat("var b = 2\n", 40)
	m.state = stateResult
	m.passed = false
	m.lastResult = &runner.Result{Output: m.challenges[0].Before}
	m.width = width
	m.height = height
	return m
}

func TestResultViewFitsTerminalHeight(t *testing.T) {
	for _, h := range []int{15, 20, 30, 50} {
		m := tallResultModel(t, 80, h)
		got := strings.Count(m.resultView(), "\n") + 1
		if got+2 > h { // +2 for the outer Padding(1,2) rows added in View
			t.Errorf("height=%d: result view is %d lines (+2 padding), want <= %d", h, got, h)
		}
	}
}

// footerGap counts the consecutive blank rows immediately above the row
// containing marker (the help line).
func footerGap(t *testing.T, view, marker string) int {
	t.Helper()
	lines := strings.Split(ansi.Strip(view), "\n")
	idx := -1
	for i, l := range lines {
		if strings.Contains(l, marker) {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("marker %q not found in view:\n%s", marker, view)
	}
	gap := 0
	for i := idx - 1; i >= 0 && strings.TrimSpace(lines[i]) == ""; i-- {
		gap++
	}
	return gap
}

func TestFooterGapIsOneBlankRowInPreviewAndResult(t *testing.T) {
	// helpStyle.MarginTop(1) alone supplies the gap above the footer; neither
	// view adds its own blank on top of it. (The menu is excluded: it pads its
	// list to a fixed height, so its gap is by design not content-flush.)
	m := tallResultModel(t, 80, 24) // failed result: diff flush above the footer
	if got := footerGap(t, m.resultView(), "r retry"); got != 1 {
		t.Errorf("result footer gap = %d blank rows, want 1", got)
	}

	m.state = statePreview
	if got := footerGap(t, m.previewView(), "enter start"); got != 1 {
		t.Errorf("preview footer gap = %d blank rows, want 1", got)
	}
}

func TestViewNeverTallerThanTerminal(t *testing.T) {
	for _, h := range []int{4, 6, 9, 12, 24} {
		for name, st := range map[string]state{"menu": stateMenu, "preview": statePreview, "result": stateResult} {
			m := tallResultModel(t, 80, h)
			m.state = st
			if got := strings.Count(m.View(), "\n") + 1; got > h {
				t.Errorf("%s at height=%d: View is %d lines, want <= %d", name, h, got, h)
			}
		}
	}
}
