package score

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdate(t *testing.T) {
	// Use a temp file for the store
	dir := t.TempDir()
	s := &Store{
		Scores: make(map[string]Entry),
		path:   filepath.Join(dir, "scores.json"),
	}

	// First score is always a record
	if !s.Update("ch1", 10, 5*time.Second) {
		t.Fatal("first score should be a record")
	}

	// Worse keystrokes: not a record
	if s.Update("ch1", 12, 3*time.Second) {
		t.Fatal("more keystrokes should not be a record")
	}

	// Fewer keystrokes: new record
	if !s.Update("ch1", 8, 10*time.Second) {
		t.Fatal("fewer keystrokes should be a record")
	}

	// Same keystrokes, faster time: new record
	if !s.Update("ch1", 8, 7*time.Second) {
		t.Fatal("same keystrokes + faster time should be a record")
	}

	// Same keystrokes, slower time: not a record
	if s.Update("ch1", 8, 9*time.Second) {
		t.Fatal("same keystrokes + slower time should not be a record")
	}

	// Verify persistence
	data, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatalf("reading score file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("score file should not be empty")
	}
}
