package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sollniss/gohelix/challenges"
	"github.com/sollniss/gohelix/config"
	"github.com/sollniss/gohelix/runner"
	"github.com/sollniss/gohelix/score"
)

// newTestModel builds a single-challenge model with score/config stores isolated
// to a temp config dir, so tests never read or write the user's real files.
func newTestModel(t *testing.T) Model {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return Model{
		challenges: []challenges.Challenge{{ID: "x", Before: "a", After: "b"}},
		scores:     score.NewStore(),
		cfg:        config.NewStore(),
		perm:       []int{0},
		permIdx:    0,
	}
}

// splitCycleOrder is the sequence one full cycle of "s" walks through, starting
// from the default (off).
var splitCycleOrder = []runner.SplitPos{runner.SplitLeft, runner.SplitRight, runner.SplitTop, runner.SplitBottom, runner.SplitOff}

func TestCycleSplitVisitsAllPositionsAndPersists(t *testing.T) {
	m := newTestModel(t)

	for _, w := range splitCycleOrder {
		m.cycleSplit()
		if got := m.splitPos(); got != w {
			t.Fatalf("cycleSplit: got %q, want %q", got, w)
		}
		// Each step must survive a restart (fresh store from the same dir).
		if got := runner.ParseSplitPos(config.NewStore().Settings.SolutionSplit); got != w {
			t.Fatalf("persisted split = %q, want %q", got, w)
		}
	}
}

func TestSplitKeyCyclesAllPositionsInPreviewAndMenu(t *testing.T) {
	keyS := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}

	for name, state := range map[string]state{"preview": statePreview, "menu": stateMenu} {
		t.Run(name, func(t *testing.T) {
			m := newTestModel(t)
			m.state = state

			var teaModel tea.Model = m
			for _, w := range splitCycleOrder {
				teaModel, _ = teaModel.Update(keyS)
				if got := teaModel.(Model).splitPos(); got != w {
					t.Fatalf("after pressing s: got %q, want %q", got, w)
				}
			}
		})
	}
}

func TestPreviewFooterShowsConfiguredPeekKey(t *testing.T) {
	m := newTestModel(t)
	m.setSplit(runner.SplitRight)
	m.cfg.Settings.PeekKey = "ctrl-g"

	footer := m.previewFooter()
	if !strings.Contains(footer, "ctrl-g peek") {
		t.Fatalf("footer should name the configured peek key:\n%s", footer)
	}
	if !strings.Contains(footer, "s split: right") {
		t.Fatalf("footer should keep the split setting:\n%s", footer)
	}

	m.setSplit(runner.SplitOff)
	if strings.Contains(m.previewFooter(), "peek") {
		t.Fatalf("peek hint should be absent when the split is off")
	}
}

func TestSplitOptionsCarryChallengeID(t *testing.T) {
	m := newTestModel(t)
	if key := m.splitOptions().SolutionKey; key != "x" {
		t.Fatalf("SolutionKey = %q, want the challenge ID", key)
	}
}

func TestSplitEnabledRunsAreAssisted(t *testing.T) {
	m := newTestModel(t)
	m.setSplit(runner.SplitLeft)

	updated, cmd := m.startChallenge()
	if cmd == nil {
		t.Fatalf("startChallenge should return an exec command")
	}
	if !updated.(Model).assisted {
		t.Fatalf("run with split enabled must be flagged assisted")
	}

	m.setSplit(runner.SplitOff)
	updated, _ = m.startChallenge()
	if updated.(Model).assisted {
		t.Fatalf("run with split off must not be flagged assisted")
	}
}
