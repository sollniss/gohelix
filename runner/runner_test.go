package runner

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	const ch, sol = "/t/challenge.go", "/t/solution.go"
	cases := []struct {
		pos   SplitPos
		args  []string
		focus string // "" = no injection (challenge is the last file, auto-focused)
	}{
		// helix focuses the last file, so left/top put the challenge last and
		// need no focus move; right/bottom must move focus back to the challenge.
		{SplitLeft, []string{"--vsplit", sol, ch}, ""},
		{SplitRight, []string{"--vsplit", ch, sol}, "\x17h"},
		{SplitTop, []string{"--hsplit", sol, ch}, ""},
		{SplitBottom, []string{"--hsplit", ch, sol}, "\x17k"},
		{SplitOff, []string{ch}, ""},
	}
	for _, tc := range cases {
		args, focus := splitArgs(tc.pos, ch, sol, DefaultWindowMoves())
		if !reflect.DeepEqual(args, tc.args) {
			t.Errorf("%s: args = %q, want %q", tc.pos, args, tc.args)
		}
		if string(focus) != tc.focus {
			t.Errorf("%s: focus = %q, want %q", tc.pos, focus, tc.focus)
		}
	}
}

func TestSplitMoves(t *testing.T) {
	cases := []struct {
		pos         SplitPos
		toSol, toCh string
	}{
		{SplitLeft, "\x17h", "\x17l"},
		{SplitRight, "\x17l", "\x17h"},
		{SplitTop, "\x17k", "\x17j"},
		{SplitBottom, "\x17j", "\x17k"},
	}
	for _, tc := range cases {
		toSol, toCh := splitMoves(tc.pos, DefaultWindowMoves())
		if string(toSol) != tc.toSol || string(toCh) != tc.toCh {
			t.Errorf("%s: moves = %q/%q, want %q/%q", tc.pos, toSol, toCh, tc.toSol, tc.toCh)
		}
	}
}

func TestWindowMovesWithDefaultsFillsOnlyUnset(t *testing.T) {
	mv := WindowMoves{Left: []byte("\x01h")}.withDefaults()
	if string(mv.Left) != "\x01h" {
		t.Errorf("custom left overwritten: %q", mv.Left)
	}
	if string(mv.Right) != "\x17l" || string(mv.Up) != "\x17k" || string(mv.Down) != "\x17j" {
		t.Errorf("unset directions should get defaults: %q %q %q", mv.Right, mv.Up, mv.Down)
	}
}

func TestParseKeySeq(t *testing.T) {
	cases := map[string]string{
		"":              "",
		"   ":           "",
		"ctrl-w h":      "\x17h",
		"ctrl+a l":      "\x01l",
		"C-w k":         "\x17k",
		"esc":           "\x1b",
		"space w h":     " wh",
		":wq enter":     ":wq\r",
		"ZZ":            "ZZ",
		"ctrl-w left":   "\x17\x1b[D",
		"tab enter":     "\t\r",
		"ctrl-w ctrl-w": "\x17\x17",
	}
	for in, want := range cases {
		if got := string(ParseKeySeq(in)); got != want {
			t.Errorf("ParseKeySeq(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQuitDetectorCustomCommands(t *testing.T) {
	// A custom binding (ZZ) is detected, and a user pair for a builtin typed
	// sequence takes precedence over the builtin injection. Injections carry a
	// leading Esc in their own chunk (clears helix's pending-key state).
	d := newQuitDetector([]QuitCommand{
		{Typed: []byte("ZZ"), Inject: []byte(":qa\r")},
		{Typed: []byte(":wq\r"), Inject: []byte(":custom-qa\r")},
	})

	got := d.inject([]byte("ZZ"))
	if join(got) != "\x1b:qa\r" || len(got) != 2 || string(got[0]) != "\x1b" {
		t.Fatalf("custom ZZ: inject = %q, want Esc chunk then :qa", got)
	}
	if got := d.inject([]byte(":wq\r")); join(got) != "\x1b:custom-qa\r" {
		t.Fatalf("user override of :wq should win: inject = %q", got)
	}
	// Builtins still work alongside the extras.
	if got := d.inject([]byte(":q\r")); join(got) != "\x1b:qa\r" {
		t.Fatalf("builtin :q should still fire: inject = %q", got)
	}
	// Empty/half pairs are ignored rather than matching everything.
	d = newQuitDetector([]QuitCommand{{Typed: nil, Inject: []byte(":qa\r")}})
	if got := d.inject([]byte("x")); got != nil {
		t.Fatalf("empty typed pattern must be ignored, injected %q", got)
	}
}

func TestEnsureModFile(t *testing.T) {
	dir := t.TempDir()
	if err := ensureModFile(dir, "gohelix.challenge"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil || string(b) != "module gohelix.challenge\n\ngo 1.21\n" {
		t.Fatalf("go.mod = %q, %v", b, err)
	}
	// Idempotent, and repairs a stale/corrupted file.
	if err := ensureModFile(dir, "gohelix.challenge"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("garbage"), 0o644)
	if err := ensureModFile(dir, "gohelix.challenge"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "go.mod")); string(b) != modFile("gohelix.challenge") {
		t.Fatalf("go.mod not repaired: %q", b)
	}
}

func TestGoWorkFile(t *testing.T) {
	if got := goWorkFile(); got != "go 1.21\n\nuse (\n\t.\n)\n" {
		t.Errorf("no extras: %q", got)
	}
	if got := goWorkFile("/home/u/.cache/gohelix/solutions/01_test"); got != "go 1.21\n\nuse (\n\t.\n\t/home/u/.cache/gohelix/solutions/01_test\n)\n" {
		t.Errorf("with solution dir: %q", got)
	}
	// Paths with spaces must be quoted to stay valid go.work syntax.
	if got := goWorkFile("/home/u/My Cache/sol"); got != "go 1.21\n\nuse (\n\t.\n\t\"/home/u/My Cache/sol\"\n)\n" {
		t.Errorf("space path not quoted: %q", got)
	}
}

func TestEnsureSolutionFile(t *testing.T) {
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	tmp := t.TempDir()

	// Creates a stable per-key dir under the cache, not the challenge temp dir.
	path, err := ensureSolutionFile("01_test", "package a\n", tmp)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(cacheHome, "gohelix", "solutions", "01_test", "solution.go")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o444 {
		t.Fatalf("stat: %v, mode %v, want 0444", err, fi.Mode().Perm())
	}

	// The dir is a real module so gopls never treats the file as orphaned.
	if b, err := os.ReadFile(filepath.Join(filepath.Dir(path), "go.mod")); err != nil || string(b) != modFile("gohelix.solution") {
		t.Fatalf("solution go.mod = %q, %v", b, err)
	}

	// A cache dir from an older gohelix version (solution.go but no go.mod)
	// gets upgraded even though the solution content is reused unchanged.
	os.Remove(filepath.Join(filepath.Dir(path), "go.mod"))
	if _, err := ensureSolutionFile("01_test", "package a\n", tmp); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "go.mod")); err != nil {
		t.Fatalf("go.mod not recreated on reuse: %v", err)
	}

	// Same content reuses the read-only file without error.
	if again, err := ensureSolutionFile("01_test", "package a\n", tmp); err != nil || again != path {
		t.Fatalf("reuse: %q, %v", again, err)
	}

	// Changed content replaces the file despite it being 0444.
	if _, err := ensureSolutionFile("01_test", "package b\n", tmp); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "package b\n" {
		t.Fatalf("content after rewrite = %q", b)
	}

	// Distinct keys get distinct dirs (one package per dir for the LSP).
	other, err := ensureSolutionFile("02_test", "package c\n", tmp)
	if err != nil || filepath.Dir(other) == filepath.Dir(path) {
		t.Fatalf("keys must not share a dir: %q vs %q (%v)", other, path, err)
	}

	// No cache dir available: falls back to a subdir of the run's temp dir —
	// still separate from the challenge file at the temp dir root.
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	fb, err := ensureSolutionFile("01_test", "package a\n", tmp)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(fb) != filepath.Join(tmp, "solution") {
		t.Fatalf("fallback dir = %q, want %q", filepath.Dir(fb), filepath.Join(tmp, "solution"))
	}
}

func TestSolutionKey(t *testing.T) {
	if got := solutionKey("01_add_error_handling", "x"); got != "01_add_error_handling" {
		t.Errorf("safe key rewritten: %q", got)
	}
	for _, unsafe := range []string{"", "../evil", "a/b", "a b"} {
		got := solutionKey(unsafe, "content")
		if got == unsafe || strings.ContainsAny(got, "/\\ .") {
			t.Errorf("unsafe key %q -> %q, want plain content hash", unsafe, got)
		}
	}
	// Hash fallback must differ for different solutions.
	if solutionKey("", "one") == solutionKey("", "two") {
		t.Errorf("distinct solutions hashed to the same dir")
	}
}

func TestEscChunks(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{":qa\r", []string{":qa\r"}},
		{"\x1b:qa\r", []string{"\x1b", ":qa\r"}},           // bare Esc isolated
		{"\x17\x1b[D", []string{"\x17\x1b[D"}},             // arrow sequence stays atomic
		{"\x1bOP", []string{"\x1bOP"}},                     // SS3 sequence stays atomic
		{"a\x1b", []string{"a\x1b"}},                       // trailing Esc ends its chunk
		{"\x1b\x1b:q\r", []string{"\x1b", "\x1b", ":q\r"}}, // double Esc
		{"\x1bx\x1b[A", []string{"\x1b", "x\x1b[A"}},       // Alt-chord risk split, CSI kept
	}

	for _, tc := range cases {
		got := escChunks([]byte(tc.in))
		if len(got) != len(tc.want) {
			t.Errorf("escChunks(%q) = %q, want %q", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if string(got[i]) != tc.want[i] {
				t.Errorf("escChunks(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

// join flattens process() chunks for assertions on the full forwarded stream.
func join(outs [][]byte) string {
	var s []byte
	for _, o := range outs {
		s = append(s, o...)
	}
	return string(s)
}

func TestParsePeekKey(t *testing.T) {
	cases := map[string]byte{
		"":        0x14, // default
		"ctrl-t":  0x14,
		"ctrl+g":  0x07,
		"C-a":     0x01,
		"CTRL-Z":  0x1a,
		"ctrl-m":  0x14, // enter: rejected, falls back
		"ctrl-i":  0x14, // tab: rejected
		"ctrl-j":  0x14, // newline: rejected
		"ctrl-h":  0x14, // backspace: rejected
		"t":       0x14, // missing modifier
		"ctrl-tt": 0x14, // not a single letter
		"ctrl-1":  0x14, // not a letter
	}
	for in, want := range cases {
		if got := ParsePeekKey(in); got != want {
			t.Errorf("ParsePeekKey(%q) = %#02x, want %#02x", in, got, want)
		}
	}
}

func TestPeekKeyName(t *testing.T) {
	if got := PeekKeyName(0x14); got != "ctrl-t" {
		t.Errorf("PeekKeyName(0x14) = %q, want ctrl-t", got)
	}
	if got := PeekKeyName(0x07); got != "ctrl-g" {
		t.Errorf("PeekKeyName(0x07) = %q, want ctrl-g", got)
	}
}

func TestPeekFilterCustomToggleKey(t *testing.T) {
	// With ctrl-g configured, ctrl-g toggles and ctrl-t is forwarded like any key.
	f := newPeekFilter(SplitRight, ParsePeekKey("ctrl-g"), DefaultWindowMoves())
	outs, counted := f.process([]byte{0x14})
	if !counted || join(outs) != "\x14" || f.peeking {
		t.Fatalf("ctrl-t must be an ordinary key here: outs=%q counted=%v peeking=%v", join(outs), counted, f.peeking)
	}
	outs, counted = f.process([]byte{0x07})
	if counted || !f.peeking || len(outs) < 2 || string(outs[1]) != "\x17l" {
		t.Fatalf("ctrl-g must toggle: outs=%q counted=%v peeking=%v", join(outs), counted, f.peeking)
	}
}

func TestPeekFilterTogglesAndCounts(t *testing.T) {
	f := newPeekFilter(SplitRight, peekToggle, DefaultWindowMoves())

	// Normal typing counts and passes through untouched.
	outs, counted := f.process([]byte("ihello"))
	if !counted || join(outs) != "ihello" || f.peeking {
		t.Fatalf("plain input: outs=%q counted=%v peeking=%v", join(outs), counted, f.peeking)
	}

	// The toggle is free: Esc chunk, then the move chunk (gap between them).
	outs, counted = f.process([]byte{peekToggle})
	if counted || !f.peeking {
		t.Fatalf("toggle must be free and enter peeking")
	}
	if len(outs) < 2 || string(outs[0]) != "\x1b" || string(outs[1]) != "\x17l" {
		t.Fatalf("toggle chunks = %q, want Esc then move-to-solution", outs)
	}

	// Keys while peeking are forwarded but free.
	outs, counted = f.process([]byte("jjkl"))
	if counted || join(outs) != "jjkl" {
		t.Fatalf("peek navigation: outs=%q counted=%v", join(outs), counted)
	}

	// Window-move and command-line keys are dropped while peeking.
	outs, counted = f.process([]byte{0x17, ':'})
	if counted || join(outs) != "" {
		t.Fatalf("blocked keys leaked: outs=%q counted=%v", join(outs), counted)
	}

	// Toggling back is free and injects the move to the challenge.
	outs, counted = f.process([]byte{peekToggle})
	if counted || f.peeking {
		t.Fatalf("exit toggle must be free and leave peeking")
	}
	if len(outs) < 2 || string(outs[0]) != "\x1b" || string(outs[1]) != "\x17h" {
		t.Fatalf("exit chunks = %q, want Esc then move-to-challenge", outs)
	}

	// Counting and ':' both work again after the peek.
	outs, counted = f.process([]byte(":wq\r"))
	if !counted || join(outs) != ":wq\r" {
		t.Fatalf("post-peek input: outs=%q counted=%v", join(outs), counted)
	}
}

func TestPeekFilterMixedRead(t *testing.T) {
	// A read carrying user bytes AND a toggle: the user bytes still count, the
	// toggle still splits chunks so the Esc can't form an Alt-chord.
	f := newPeekFilter(SplitBottom, peekToggle, DefaultWindowMoves())
	outs, counted := f.process(append([]byte("x"), peekToggle))
	if !counted {
		t.Fatalf("the x before the toggle must count")
	}
	if len(outs) < 2 || string(outs[0]) != "x\x1b" || string(outs[1]) != "\x17j" {
		t.Fatalf("chunks = %q", outs)
	}
}

func TestParseSplitPos(t *testing.T) {
	cases := map[string]SplitPos{
		"right":   SplitRight,
		" Left ":  SplitLeft,
		"TOP":     SplitTop,
		"bottom":  SplitBottom,
		"off":     SplitOff,
		"":        SplitOff,
		"garbage": SplitOff,
	}
	for in, want := range cases {
		if got := ParseSplitPos(in); got != want {
			t.Errorf("ParseSplitPos(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQuitDetectorDetectsBuiltinCommands(t *testing.T) {
	cases := []struct {
		name  string
		typed string // bytes fed one at a time, as a human types them
		want  string // expected injection ("" = none)
	}{
		{"write-quit", ":wq\r", "\x1b:qa\r"},
		{"quit", ":q\r", "\x1b:qa\r"},
		{"force-quit", ":q!\r", "\x1b:qa!\r"},
		{"force-write-quit", ":wq!\r", "\x1b:qa!\r"},
		{"x-alias", ":x\r", "\x1b:qa\r"},
		{"force-x", ":x!\r", "\x1b:qa!\r"},
		{"save-only", ":w\r", ""},     // saving must not trigger a quit
		{"no-enter-yet", ":wq", ""},   // command not submitted
		{"unrelated", "iabc\x1b", ""}, // ordinary editing
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newQuitDetector(nil)
			var got string
			// Feed byte-by-byte so we exercise matching across reads.
			for i := 0; i < len(tc.typed); i++ {
				if inj := d.inject([]byte(tc.typed[i : i+1])); inj != nil {
					got = join(inj)
				}
			}
			if got != tc.want {
				t.Fatalf("typed %q: got inject %q, want %q", tc.typed, got, tc.want)
			}
		})
	}
}

func TestQuitDetectorFiresOncePerCommand(t *testing.T) {
	d := newQuitDetector(nil)
	var fired int
	feed := func(s string) {
		for i := 0; i < len(s); i++ {
			if d.inject([]byte(s[i:i+1])) != nil {
				fired++
			}
		}
	}
	feed(":wq\r")
	if fired != 1 {
		t.Fatalf("expected exactly one injection, got %d", fired)
	}
	// The trailing \r left in the buffer must not re-trigger on the next keystroke.
	feed("j")
	if fired != 1 {
		t.Fatalf("injection fired again after the command completed: %d", fired)
	}
}
