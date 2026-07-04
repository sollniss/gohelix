package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sollniss/gohelix/runner"
)

var (
	keyL = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	keyH = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
)

func TestPreviewHScrollStopsAtWidestLine(t *testing.T) {
	m := newTestModel(t)
	m.state = statePreview
	m.width = 48                                     // colWidth = 20
	m.challenges[0].Before = strings.Repeat("a", 30) // widest line: max offset 10
	m.challenges[0].After = "b"

	var tm tea.Model = m
	for range 10 {
		tm, _ = tm.Update(keyL)
	}
	if got := tm.(Model).previewScroll.x; got != 10 {
		t.Fatalf("previewScroll.x = %d, want clamped to 10", got)
	}

	// Scrolling back reaches 0 in three presses (10 → 6 → 2 → 0), instead of
	// having to undo every overshot press one by one.
	for range 3 {
		tm, _ = tm.Update(keyH)
	}
	if got := tm.(Model).previewScroll.x; got != 0 {
		t.Fatalf("previewScroll.x after scrolling back = %d, want 0", got)
	}
}

func TestResultHScrollStopsAtWidestLine(t *testing.T) {
	m := newTestModel(t)
	m.state = stateResult
	m.width = 48 // colWidth = 20
	m.passed = false
	m.lastResult = &runner.Result{Output: strings.Repeat("x", 30) + "\n"} // max offset 10

	var tm tea.Model = m
	for range 10 {
		tm, _ = tm.Update(keyL)
	}
	if got := tm.(Model).resultScroll.x; got != 10 {
		t.Fatalf("resultScroll.x = %d, want clamped to 10", got)
	}
}

func TestResultHScrollStaysZeroWhenPassed(t *testing.T) {
	// A passed result shows no diff, so there is nothing to scroll.
	m := newTestModel(t)
	m.state = stateResult
	m.width = 48
	m.passed = true
	m.lastResult = &runner.Result{Output: strings.Repeat("x", 30) + "\n"}

	tm, _ := tea.Model(m).Update(keyL)
	if got := tm.(Model).resultScroll.x; got != 0 {
		t.Fatalf("resultScroll.x = %d, want 0 on a passed result", got)
	}
}
