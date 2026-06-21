package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/sollniss/gohelix/challenges"
	"github.com/sollniss/gohelix/score"
)

func menuModel(height int) Model {
	chs := make([]challenges.Challenge, 20)
	for i := range chs {
		chs[i] = challenges.Challenge{ID: fmt.Sprintf("ch%02d", i), Title: fmt.Sprintf("Title %d", i)}
	}
	return Model{
		challenges: chs,
		scores:     score.NewStore(),
		state:      stateMenu,
		width:      80,
		height:     height,
	}
}

func TestMenuViewKeepsHeaderAndFooterVisible(t *testing.T) {
	m := menuModel(12)
	out := ansi.Strip(m.menuView())

	// Header and footer are always rendered, regardless of list length.
	if !strings.Contains(out, menuTitle) {
		t.Errorf("menu header missing:\n%s", out)
	}
	if !strings.Contains(out, menuHelp) {
		t.Errorf("menu footer missing:\n%s", out)
	}
	// The list is windowed to fit the height: at the top it shows challenge 1 but
	// not challenge 6 (only menuVisibleRows rows fit).
	if !strings.Contains(out, " 1. Title 0") {
		t.Errorf("expected first challenge at top:\n%s", out)
	}
	visible := m.menuVisibleRows()
	if strings.Contains(out, fmt.Sprintf("%2d. Title %d", visible+1, visible)) {
		t.Errorf("challenge beyond the window should be hidden (visible=%d):\n%s", visible, out)
	}
}

func TestMenuViewFollowsCursorToBottom(t *testing.T) {
	m := menuModel(12)
	m.menuCursor = len(m.challenges) - 1
	m.clampMenuScroll()
	out := ansi.Strip(m.menuView())

	// The selected (last) challenge is visible after the window scrolls down.
	if !strings.Contains(out, "20. Title 19") {
		t.Errorf("expected last challenge visible:\n%s", out)
	}
	// The header stays pinned even when scrolled.
	if !strings.Contains(out, menuTitle) {
		t.Errorf("header should remain visible when scrolled:\n%s", out)
	}
	// Challenge 1 has scrolled out of view.
	if strings.Contains(out, " 1. Title 0") {
		t.Errorf("first challenge should be scrolled away:\n%s", out)
	}
}

func TestMenuViewFitsTerminalHeight(t *testing.T) {
	// The rendered menu must never be taller than the terminal (the bug: the list
	// overflowed and pushed the header off the top).
	for _, h := range []int{8, 12, 24, 60} {
		m := menuModel(h)
		got := strings.Count(m.menuView(), "\n") + 1
		// Body height excludes the outer Padding(1,2) applied in View (2 rows).
		if got > h-2 {
			t.Errorf("height=%d: menu rendered %d lines, want <= %d", h, got, h-2)
		}
	}
}
