package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// QuitCommand pairs a key sequence that quits helix with the sequence to
// inject after it so the solution window closes too. Both sides use the same
// notation as the window_move_* settings, e.g. {"typed": "ZZ", "inject":
// ":qa enter"}. The injection is automatically preceded by Esc, so key-bound
// quits that leave helix's pending-key state active still work.
type QuitCommand struct {
	Typed  string `json:"typed"`
	Inject string `json:"inject"`
}

// Settings holds user preferences persisted between runs.
type Settings struct {
	// DiffMode is the diff view used on the preview and result screens:
	// "builtin" (bat side-by-side) or "difft" (difft-aligned columns, applied
	// only when difft is on PATH). The "d" key toggles it in the TUI; the last
	// choice is remembered here.
	DiffMode string `json:"diff_mode,omitempty"`

	// SolutionSplit is where the read-only solution window opens during a
	// challenge: "off", "left", "right", "top" or "bottom".
	SolutionSplit string `json:"solution_split,omitempty"`
	// PeekKey is the control key that toggles the free peek at the solution
	// during split runs, e.g. "ctrl-t" (the default).
	PeekKey string `json:"peek_key,omitempty"`

	// WindowMove* override the key sequences gohelix injects to move helix's
	// window focus, for users who remapped helix's window-mode keys. Sequences
	// are space-separated tokens: "ctrl-x" for control keys, "esc", "space",
	// "tab", "enter", arrow names ("up", "left", ...), anything else is typed
	// literally. Defaults: "ctrl-w h" / "ctrl-w l" / "ctrl-w k" / "ctrl-w j".
	WindowMoveLeft  string `json:"window_move_left,omitempty"`
	WindowMoveRight string `json:"window_move_right,omitempty"`
	WindowMoveUp    string `json:"window_move_up,omitempty"`
	WindowMoveDown  string `json:"window_move_down,omitempty"`

	// QuitCommands are extra quit detections for users who quit helix through
	// remapped keys or custom commands. They are checked before the builtin
	// :q/:wq/:x family; an entry whose typed sequence equals a builtin one
	// overrides the builtin injection.
	QuitCommands []QuitCommand `json:"quit_commands,omitempty"`
}

// Store persists settings to ~/.config/gohelix/config.json.
type Store struct {
	Settings Settings
	// LoadErr is set when config.json exists but could not be read or parsed
	// (typically a typo in a hand-edited file). Whatever did parse is used for
	// the session, and Save becomes a no-op so the user's file is never
	// overwritten with a partial or default configuration.
	LoadErr error
	path    string
}

// NewStore loads or creates the settings store. Like the score store, it
// degrades to in-memory-only when no config dir is available.
func NewStore() *Store {
	s := &Store{}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return s // work without persistence
	}

	dir := filepath.Join(configDir, "gohelix")
	_ = os.MkdirAll(dir, 0o755)
	s.path = filepath.Join(dir, "config.json")

	data, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// first run: nothing to load
	case err != nil:
		s.LoadErr = err
	case len(bytes.TrimSpace(data)) > 0: // an empty file is just "no settings"
		s.LoadErr = json.Unmarshal(data, &s.Settings)
	}

	return s
}

// Save writes the settings to disk, best-effort. It does nothing when the
// existing file failed to load (see LoadErr): saving then would replace the
// user's settings with whatever survived the failed parse.
func (s *Store) Save() {
	if s.path == "" || s.LoadErr != nil {
		return
	}
	data, err := json.MarshalIndent(s.Settings, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, data, 0o644)
}
