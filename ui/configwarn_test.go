package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPreviewWarnsAboutBrokenConfig(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 120, 30
	m.state = statePreview

	if out := ansi.Strip(m.previewView()); strings.Contains(out, "config.json") {
		t.Fatalf("warning shown for a healthy config:\n%s", out)
	}
	rows := m.previewBodyRows()

	m.cfg.LoadErr = errors.New("invalid character '}'")
	out := ansi.Strip(m.previewView())
	if !strings.Contains(out, "config.json error") || !strings.Contains(out, "invalid character '}'") {
		t.Fatalf("broken config not reported in the preview:\n%s", out)
	}
	// The warning takes its rows from the body, not from the terminal.
	if got := m.previewBodyRows(); got != rows-2 {
		t.Errorf("body rows with warning = %d, want %d", got, rows-2)
	}
}
