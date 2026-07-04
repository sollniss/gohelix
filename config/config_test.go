package config

import "testing"

func TestStoreRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	s := NewStore()
	if s.Settings.SolutionSplit != "" {
		t.Fatalf("fresh store should have empty settings, got %q", s.Settings.SolutionSplit)
	}

	s.Settings.SolutionSplit = "top"
	s.Save()

	reloaded := NewStore()
	if reloaded.Settings.SolutionSplit != "top" {
		t.Fatalf("reloaded settings = %+v, want solution_split = top", reloaded.Settings)
	}
}
