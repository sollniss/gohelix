package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sollniss/gohelix/difft"
	"github.com/sollniss/gohelix/runner"
)

func TestLeavingResultForPickerDropsResultAlignment(t *testing.T) {
	// The alignment on a failed result pairs "your solution" with "expected".
	// The picker only leads back to a preview (before vs after), where those
	// rows would hide most of the Before column, so leaving the result must
	// drop them and request the preview's alignment.
	m := newTestModel(t)
	m.difftAvailable = true
	m.diffMode = DiffDifft
	m.state = stateResult
	m.lastResult = &runner.Result{Output: "something else"}
	m.difftResult = difft.Result{Rows: [][2]int{{0, 0}, {-1, 1}}}
	seq := m.diffSeq

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.state != stateMenu {
		t.Fatalf("state = %v, want the picker", m.state)
	}
	if len(m.difftResult.Rows) != 0 {
		t.Errorf("result alignment survived into the picker: %v", m.difftResult.Rows)
	}
	if cmd == nil || m.diffSeq == seq {
		t.Errorf("no fresh alignment requested (cmd nil: %v, diffSeq %d -> %d)", cmd == nil, seq, m.diffSeq)
	}

	// Back out of the picker: the preview must not render with stale rows.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.state != statePreview || len(m.difftResult.Rows) != 0 {
		t.Errorf("state = %v, rows = %v; want a preview without stale rows", m.state, m.difftResult.Rows)
	}
}
