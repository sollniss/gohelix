package ui

import "testing"

// rows models a difft alignment where the right side gains two inserted lines:
//
//	row | left | right
//	  0 |   0  |   0
//	  1 |   1  |   1
//	  2 |  -1  |   2   (insertion: no left line)
//	  3 |  -1  |   3   (insertion: no left line)
//	  4 |   2  |   4
//	  5 |   3  |   5
var rows = [][2]int{{0, 0}, {1, 1}, {-1, 2}, {-1, 3}, {2, 4}, {3, 5}}

func TestScrollRoundTripBuiltin(t *testing.T) {
	// Builtin is positional: content row i shows left line i, so a scroll maps to
	// its own offset minus the header and back unchanged.
	for scroll := diffHeaderRows; scroll < 10; scroll++ {
		line := scrollToLeftLine(DiffBuiltin, nil, scroll)
		if got := leftLineToScroll(DiffBuiltin, nil, line); got != scroll {
			t.Errorf("builtin round trip scroll=%d: line=%d back=%d", scroll, line, got)
		}
	}
}

func TestScrollHeaderRegionAnchorsToTop(t *testing.T) {
	// While the header is visible both modes are at the top, so anchor to line 0.
	for scroll := 0; scroll < diffHeaderRows; scroll++ {
		if got := scrollToLeftLine(DiffDifft, rows, scroll); got != 0 {
			t.Errorf("scroll=%d (header visible) anchored to line %d, want 0", scroll, got)
		}
	}
}

func TestScrollLeftLineDifftMapping(t *testing.T) {
	// Left line 2 lives on difft row 4 (after the two insertions), so its scroll
	// offset is row 4 + the 2 header rows = 6.
	if got := leftLineToScroll(DiffDifft, rows, 2); got != 6 {
		t.Errorf("leftLineToScroll(difft, line 2) = %d, want 6", got)
	}
	// And the inverse: scroll 6 sits on left line 2.
	if got := scrollToLeftLine(DiffDifft, rows, 6); got != 2 {
		t.Errorf("scrollToLeftLine(difft, scroll 6) = %d, want 2", got)
	}
}

func TestScrollInsertionRowWalksBackToLeftLine(t *testing.T) {
	// Difft rows 2 and 3 are right-only insertions (left index -1). A viewport
	// topped at one of them anchors to the nearest preceding real left line (1).
	for _, scroll := range []int{4, 5} { // rows 2 and 3 plus the header offset
		if got := scrollToLeftLine(DiffDifft, rows, scroll); got != 1 {
			t.Errorf("scrollToLeftLine(difft, scroll %d) = %d, want 1", scroll, got)
		}
	}
}

func TestScrollCrossModePreservesLeftLine(t *testing.T) {
	// The toggle path: read the top left line in one mode, translate to the other.
	// Builtin scroll 4 shows left line 2; in difft that line sits at scroll 6.
	line := scrollToLeftLine(DiffBuiltin, nil, 4)
	if line != 2 {
		t.Fatalf("builtin scroll 4 -> left line %d, want 2", line)
	}
	if got := leftLineToScroll(DiffDifft, rows, line); got != 6 {
		t.Errorf("difft scroll for left line %d = %d, want 6", line, got)
	}
}
