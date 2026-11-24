package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	s := NewStore()
	if s.Settings.SolutionSplit != "" {
		t.Fatalf("fresh store should have empty settings, got %q", s.Settings.SolutionSplit)
	}
	if s.LoadErr != nil {
		t.Fatalf("a missing config file is not an error, got %v", s.LoadErr)
	}

	s.Settings.SolutionSplit = "top"
	s.Save()

	reloaded := NewStore()
	if reloaded.Settings.SolutionSplit != "top" {
		t.Fatalf("reloaded settings = %+v, want solution_split = top", reloaded.Settings)
	}
}

// writeConfig puts content into the config file NewStore will read and
// returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "gohelix", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBrokenConfigIsReportedAndNeverOverwritten(t *testing.T) {
	cases := map[string]string{
		// A hand-edited file with a trailing comma: nothing parses.
		"syntax error": "{\n  \"peek_key\": \"ctrl-g\",\n  \"quit_commands\": [{\"typed\": \"ZZ\", \"inject\": \":qa enter\"}],\n}\n",
		// A wrong type: the other fields parse, saving would drop this one.
		"type error": "{\"peek_key\": 5, \"solution_split\": \"left\"}",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, content)

			s := NewStore()
			if s.LoadErr == nil {
				t.Fatalf("LoadErr = nil, want the parse error")
			}

			// What the in-app toggles do: change a setting and save.
			s.Settings.SolutionSplit = "top"
			s.Save()

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != content {
				t.Fatalf("broken config was overwritten:\n%s", got)
			}
		})
	}
}

func TestEmptyConfigFileIsNotAnError(t *testing.T) {
	path := writeConfig(t, " \n")

	s := NewStore()
	if s.LoadErr != nil {
		t.Fatalf("LoadErr = %v, want nil for an empty file", s.LoadErr)
	}
	s.Settings.DiffMode = "difft"
	s.Save()
	if got, _ := os.ReadFile(path); len(got) < 10 {
		t.Fatalf("settings were not saved over an empty file: %q", got)
	}
}
