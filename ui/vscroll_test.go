package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/sollniss/gohelix/runner"
)

var keyJ = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}

// numberedLines returns n lines "prefix00".."prefixNN", one per line.
func numberedLines(prefix string, n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "%s%02d\n", prefix, i)
	}
	return b.String()
}

// The scroll limit is computed from diffBodyLines' prediction of the rendered
// body, not the rendered body itself. These tests pin that the prediction is
// not short: scrolling all the way down must bring the very last line on
// screen.

func TestPreviewScrollReachesLastLine(t *testing.T) {
	m := newTestModel(t)
	m.state = statePreview
	m.width = 80
	m.height = 20
	m.challenges[0].Before = numberedLines("left", 40)
	m.challenges[0].After = "right\n"

	var tm tea.Model = m
	for range 100 {
		tm, _ = tm.Update(keyJ)
	}
	if view := ansi.Strip(tm.(Model).View()); !strings.Contains(view, "left39") {
		t.Fatalf("last line not reachable by scrolling:\n%s", view)
	}
}

func TestResultScrollReachesLastLine(t *testing.T) {
	m := newTestModel(t)
	m.state = stateResult
	m.width = 80
	m.height = 20
	m.passed = false
	m.lastResult = &runner.Result{Output: numberedLines("out", 40)}

	var tm tea.Model = m
	for range 100 {
		tm, _ = tm.Update(keyJ)
	}
	if view := ansi.Strip(tm.(Model).View()); !strings.Contains(view, "out39") {
		t.Fatalf("last line not reachable by scrolling:\n%s", view)
	}
}
