package ui

import (
	"testing"

	"github.com/sollniss/gohelix/config"
)

func TestParseDiffMode(t *testing.T) {
	cases := []struct {
		in   string
		want DiffMode
	}{
		{"", DiffBuiltin},
		{"builtin", DiffBuiltin},
		{"difft", DiffDifft},
		{" DIFFT ", DiffDifft},
		{"nonsense", DiffBuiltin},
	}
	for _, c := range cases {
		if got := parseDiffMode(c.in); got != c.want {
			t.Errorf("parseDiffMode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStartupDiffMode(t *testing.T) {
	// A stored difft preference only applies when difft is actually available.
	if got := startupDiffMode("difft", true); got != DiffDifft {
		t.Errorf("difft stored and available: got %q, want %q", got, DiffDifft)
	}
	if got := startupDiffMode("difft", false); got != DiffBuiltin {
		t.Errorf("difft stored but unavailable: got %q, want %q", got, DiffBuiltin)
	}
	if got := startupDiffMode("", true); got != DiffBuiltin {
		t.Errorf("nothing stored: got %q, want %q", got, DiffBuiltin)
	}
}

func TestToggleDiffModePersistsAcrossRestart(t *testing.T) {
	m := newTestModel(t)
	m.difftAvailable = true

	if cmd := m.toggleDiffMode(); cmd == nil {
		t.Fatalf("switching to difft should request an alignment")
	}
	if m.diffMode != DiffDifft {
		t.Fatalf("diffMode = %q, want %q", m.diffMode, DiffDifft)
	}
	// The choice must survive a restart (fresh store from the same dir).
	if got := config.NewStore().Settings.DiffMode; got != string(DiffDifft) {
		t.Fatalf("persisted diff mode = %q, want %q", got, DiffDifft)
	}

	if cmd := m.toggleDiffMode(); cmd != nil {
		t.Fatalf("switching back to builtin should not request an alignment")
	}
	if got := config.NewStore().Settings.DiffMode; got != string(DiffBuiltin) {
		t.Fatalf("persisted diff mode = %q, want %q", got, DiffBuiltin)
	}
}

func TestToggleDiffModeNoopWithoutDifft(t *testing.T) {
	m := newTestModel(t)
	m.difftAvailable = false

	if cmd := m.toggleDiffMode(); cmd != nil {
		t.Fatalf("toggle without difft should be a no-op")
	}
	if m.diffMode == DiffDifft {
		t.Fatalf("toggle without difft must not enable difft")
	}
	// The stored setting is untouched, so a preference from another machine
	// isn't clobbered here.
	if got := config.NewStore().Settings.DiffMode; got != "" {
		t.Fatalf("persisted diff mode = %q, want empty", got)
	}
}

func TestInitRequestsDiffOnlyInDifftMode(t *testing.T) {
	m := newTestModel(t)

	m.diffMode = DiffDifft
	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("Init in difft mode should request the initial alignment")
	}
	if _, ok := cmd().(requestDiffMsg); !ok {
		t.Fatalf("Init command should produce a requestDiffMsg")
	}

	m.diffMode = DiffBuiltin
	if m.Init() != nil {
		t.Fatalf("Init in builtin mode should be a no-op")
	}
}
