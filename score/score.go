package score

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Entry holds the best scores for a single challenge.
type Entry struct {
	Keystrokes int           `json:"keystrokes"`
	Duration   time.Duration `json:"duration"`
}

// Store persists best scores to ~/.config/gohelix/scores.json.
type Store struct {
	Scores map[string]Entry `json:"scores"`
	path   string
}

// NewStore loads or creates the score store.
func NewStore() *Store {
	s := &Store{
		Scores: make(map[string]Entry),
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return s // work without persistence
	}

	dir := filepath.Join(configDir, "gohelix")
	_ = os.MkdirAll(dir, 0o755)
	s.path = filepath.Join(dir, "scores.json")

	data, err := os.ReadFile(s.path)
	if err == nil {
		_ = json.Unmarshal(data, &s.Scores)
	}

	return s
}

// Get returns the best score for a challenge, if any.
func (s *Store) Get(id string) (Entry, bool) {
	e, ok := s.Scores[id]
	return e, ok
}

// Update records a new score if it beats the existing record.
// Returns true if a new record was set.
// Records are ranked by fewest keystrokes first, then fastest time.
func (s *Store) Update(id string, keystrokes int, duration time.Duration) bool {
	existing, ok := s.Scores[id]
	if !ok || keystrokes < existing.Keystrokes ||
		(keystrokes == existing.Keystrokes && duration < existing.Duration) {
		s.Scores[id] = Entry{Keystrokes: keystrokes, Duration: duration}
		s.save()
		return true
	}
	return false
}

func (s *Store) save() {
	if s.path == "" {
		return
	}
	data, err := json.MarshalIndent(s.Scores, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, data, 0o644)
}
