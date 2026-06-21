package difft

import (
	"os"
	"strings"
	"testing"
)

// TestAlign verifies that the package can drive difftastic in JSON mode and
// parse its line alignment and change spans. Skips when difft is not installed.
func TestAlign(t *testing.T) {
	before, err := os.ReadFile("../challenges/testdata/03_add_context/before.go")
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile("../challenges/testdata/03_add_context/after.go")
	if err != nil {
		t.Fatal(err)
	}

	res, err := Align("difft", string(before), string(after))
	if err != nil {
		t.Skipf("difft unavailable: %v", err)
	}
	if len(res.Rows) == 0 {
		t.Fatal("no aligned rows parsed")
	}

	// This change is pure insertions/deletions, so there must be at least one row
	// with a blank side (-1).
	sawInsertion := false
	for _, row := range res.Rows {
		if row[0] == -1 || row[1] == -1 {
			sawInsertion = true
		}
		if row[0] < -1 || row[1] < -1 {
			t.Fatalf("invalid row %v", row)
		}
	}
	if !sawInsertion {
		t.Error("expected at least one insertion/deletion row with a blank side")
	}

	// The "after" side gains context.Context, so there must be change spans.
	if len(res.Rhs) == 0 {
		t.Error("expected rhs change spans for a modified diff")
	}
}

// TestAlignParseErrorFallback verifies that invalid source (which makes difft
// fall back to its line-based Text mode) still yields a usable diff via difft's
// own fallback, rather than an error or empty result.
func TestAlignParseErrorFallback(t *testing.T) {
	if !Available() {
		t.Skip("difft unavailable")
	}
	after, err := os.ReadFile("../challenges/testdata/03_add_context/after.go")
	if err != nil {
		t.Fatal(err)
	}
	// Break the package keyword so the file no longer parses as Go.
	broken := strings.Replace(string(after), "package example", "kage example", 1)

	res, err := Align("difft", broken, string(after))
	if err != nil {
		t.Fatalf("expected best-effort result, got error: %v", err)
	}
	if len(res.Rows) == 0 {
		t.Error("expected a usable line alignment even in difft's text-mode fallback")
	}
}
