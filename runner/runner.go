package runner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	pty "github.com/aymanbagabas/go-pty"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"
)

// QuitCommand pairs a byte sequence the user produces to quit helix with the
// bytes to inject right after it so the read-only solution window closes too.
// In split mode helix's :wq/:q only closes the focused window, leaving the
// solution open and helix running.
type QuitCommand struct {
	Typed  []byte
	Inject []byte
}

// defaultQuitCommands covers helix's builtin quit spellings.
var defaultQuitCommands = []QuitCommand{
	{[]byte(":wq!\r"), []byte(":qa!\r")},
	{[]byte(":q!\r"), []byte(":qa!\r")},
	{[]byte(":x!\r"), []byte(":qa!\r")},
	{[]byte(":wq\r"), []byte(":qa\r")},
	{[]byte(":x\r"), []byte(":qa\r")},
	{[]byte(":q\r"), []byte(":qa\r")},
}

// quitDetector watches the forwarded input stream for quit commands. Extra
// user-configured commands are checked before the builtin ones, so a user pair
// with the same Typed sequence overrides the builtin injection.
type quitDetector struct {
	commands []QuitCommand
	prefix   []byte // injected ahead of every quit, see newQuitDetector
	tailCap  int
	tail     []byte // rolling buffer of recent forwarded bytes
}

// newQuitDetector builds the detector for a split whose windows are reached
// with the given focus moves (see splitMoves).
//
// Every injection starts by reverting the solution buffer: nothing stops the
// user from editing it (while peeking, or after clicking into it), and a
// modified solution makes the injected quit-all fail with "unsaved buffer"
// right after the challenge was saved and closed. So focus goes to the
// solution, :reload discards its changes, and focus returns to the challenge
// before the quit itself. The quit stays non-forcing on purpose: when the
// user's own quit was refused (unsaved challenge) or the match was a false
// positive (":wq" typed in insert mode), the injected one is refused as well
// and nothing is lost. The reload cannot hit the challenge: with two windows
// the move lands on the solution, and with one the challenge is already gone.
//
// The Esc in front comes first because a quit executed through a key binding
// (e.g. a ZZ minor mode) leaves helix's pending-key state active, which
// silently swallows the next key; Esc clears it, and is a no-op after a quit
// typed at the command line.
func newQuitDetector(extra []QuitCommand, toSolution, toChallenge []byte) *quitDetector {
	cmds := make([]QuitCommand, 0, len(extra)+len(defaultQuitCommands))
	for _, c := range extra {
		if len(c.Typed) > 0 && len(c.Inject) > 0 {
			cmds = append(cmds, c)
		}
	}
	cmds = append(cmds, defaultQuitCommands...)
	maxLen := 0
	for _, c := range cmds {
		maxLen = max(maxLen, len(c.Typed))
	}
	prefix := []byte{0x1b}
	prefix = append(prefix, toSolution...)
	prefix = append(prefix, ":reload\r"...)
	prefix = append(prefix, toChallenge...)
	return &quitDetector{commands: cmds, prefix: prefix, tailCap: maxLen + 4}
}

// inject appends the newly-forwarded input to the rolling tail and, if it now
// ends with a recognized quit command, returns the chunks to inject (written
// with escGap pauses between them) and clears the tail so the same command
// can't fire twice. It returns nil otherwise.
func (d *quitDetector) inject(in []byte) [][]byte {
	d.tail = append(d.tail, in...)
	if len(d.tail) > d.tailCap {
		d.tail = d.tail[len(d.tail)-d.tailCap:]
	}
	for _, c := range d.commands {
		if bytes.HasSuffix(d.tail, c.Typed) {
			d.tail = d.tail[:0]
			seq := append([]byte(nil), d.prefix...)
			return escChunks(append(seq, c.Inject...))
		}
	}
	return nil
}

// escChunks splits an injection byte sequence after every bare Esc, so the
// writer can pause between chunks: helix parses Esc immediately followed by
// another byte in the same write as an Alt-chord instead of two key presses.
// An Esc opening a terminal escape sequence (\x1b[ or \x1bO, e.g. arrow keys)
// must stay with its sequence and is not split.
func escChunks(seq []byte) [][]byte {
	var chunks [][]byte
	start := 0
	for i, b := range seq {
		if b == 0x1b && (i+1 >= len(seq) || (seq[i+1] != '[' && seq[i+1] != 'O')) {
			chunks = append(chunks, seq[start:i+1])
			start = i + 1
		}
	}
	if start < len(seq) {
		chunks = append(chunks, seq[start:])
	}
	return chunks
}

// ParseKeySeq turns a config key-sequence string into the bytes helix receives.
func ParseKeySeq(s string) []byte {
	var out []byte
	for _, tok := range strings.Fields(s) {
		lower := strings.ToLower(tok)
		switch lower {
		case "esc":
			out = append(out, 0x1b)
			continue
		case "space":
			out = append(out, ' ')
			continue
		case "tab":
			out = append(out, '\t')
			continue
		case "enter", "cr", "return":
			out = append(out, '\r')
			continue
		case "up":
			out = append(out, 0x1b, '[', 'A')
			continue
		case "down":
			out = append(out, 0x1b, '[', 'B')
			continue
		case "right":
			out = append(out, 0x1b, '[', 'C')
			continue
		case "left":
			out = append(out, 0x1b, '[', 'D')
			continue
		}
		if ctrl, ok := parseCtrlToken(lower); ok {
			out = append(out, ctrl)
			continue
		}
		out = append(out, tok...)
	}
	return out
}

// parseCtrlToken maps "ctrl-x"/"ctrl+x"/"c-x" to its control byte.
func parseCtrlToken(tok string) (byte, bool) {
	for _, prefix := range []string{"ctrl-", "ctrl+", "c-"} {
		if rest, ok := strings.CutPrefix(tok, prefix); ok && len(rest) == 1 {
			if c := rest[0]; c >= 'a' && c <= 'z' {
				return c - 'a' + 1, true
			}
		}
	}
	return 0, false
}

// keyCounter splits the counted input stream into key events.
type keyCounter struct {
	pending  []byte // incomplete escape sequence or rune from the previous read
	inString bool   // inside an OSC/DCS-style string, skipping to its terminator
}

// Bracketed-paste markers (mode 2004): sent by the terminal around pasted
// text, never pressed by the user, so they don't count. The pasted content
// between them does.
var (
	pasteStart = []byte("\x1b[200~")
	pasteEnd   = []byte("\x1b[201~")
)

// maxPending bounds the carried partial sequence. Real terminal sequences are
// far shorter; anything longer is counted as one garbage event and dropped so
// a malformed stream can't wedge the counter.
const maxPending = 32

// count returns the number of key events in in, resuming any partial sequence
// carried from the previous read.
func (k *keyCounter) count(in []byte) int {
	if len(in) == 0 {
		return 0
	}
	buf := in
	if len(k.pending) > 0 {
		buf = append(k.pending, in...)
		k.pending = nil
	}
	events := 0
	for i := 0; i < len(buf); {
		if k.inString {
			n, closed := skipString(buf[i:])
			i += n
			if !closed {
				// An unterminated string may end in a lone Esc that is the
				// first half of a split ST; carry it into the next read.
				if i < len(buf) {
					k.pending = append([]byte(nil), buf[i:]...)
				}
				return events
			}
			k.inString = false
			continue
		}
		// OSC/DCS/APC/PM/SOS introducers open a terminal string response
		// (color lookups, XTVersion, ...), free until its BEL/ST terminator.
		if buf[i] == 0x1b && i+2 < len(buf) {
			switch buf[i+1] {
			case ']', 'P', 'X', '^', '_':
				k.inString = true
				i += 2
				continue
			}
		}
		n, counts, incomplete := parseKeyEvent(buf[i:])
		if incomplete {
			if len(buf)-i > maxPending {
				events++ // garbage, not a key sequence: one event, dropped
			} else {
				k.pending = append([]byte(nil), buf[i:]...)
			}
			break
		}
		if counts {
			events++
		}
		i += n
	}
	return events
}

// skipString advances through an OSC/DCS-style string until its BEL or ST
// (Esc \) terminator, returning the bytes consumed and whether the terminator
// was found. A trailing lone Esc is left unconsumed so a split ST can
// complete on the next read.
func skipString(b []byte) (n int, closed bool) {
	for j := 0; j < len(b); j++ {
		switch b[j] {
		case 0x07: // BEL
			return j + 1, true
		case 0x1b:
			if j+1 >= len(b) {
				return j, false // possible first half of a split ST
			}
			if b[j+1] == '\\' {
				return j + 2, true
			}
		}
	}
	return len(b), false
}

// parseKeyEvent reports the byte length of the first key event in b and
// whether it counts as a keystroke. incomplete means b ends before the event
// does (a sequence or rune cut off at the read boundary).
func parseKeyEvent(b []byte) (n int, counts, incomplete bool) {
	if b[0] != 0x1b {
		if !utf8.FullRune(b) {
			return 0, false, true // rune cut off at the read boundary
		}
		_, size := utf8.DecodeRune(b)
		return size, true, false
	}
	if len(b) == 1 {
		// A lone Esc is the Esc key, not a sequence prefix: terminals write a
		// key's bytes in one go, so a sequence split right after its Esc
		// effectively doesn't happen.
		return 1, true, false
	}
	switch b[1] {
	case '[': // CSI: \x1b[ params(0x30–0x3f) intermediates(0x20–0x2f) final(0x40–0x7e)
		i := 2
		for i < len(b) && b[i] >= 0x30 && b[i] <= 0x3f {
			i++
		}
		for i < len(b) && b[i] >= 0x20 && b[i] <= 0x2f {
			i++
		}
		if i == len(b) {
			return 0, false, true // final byte not read yet
		}
		if b[i] < 0x40 || b[i] > 0x7e {
			// No valid final byte: count the malformed introducer as one
			// event and resync at the offending byte.
			return i, true, false
		}
		seq := b[:i+1]
		if bytes.Equal(seq, pasteStart) || bytes.Equal(seq, pasteEnd) {
			return i + 1, false, false
		}
		return i + 1, csiIsKey(b[2:i], b[i]), false
	case 'O': // SS3, e.g. F1–F4: \x1bO + one byte
		if len(b) == 2 {
			return 0, false, true
		}
		return 3, true, false
	default:
		// Esc + rune in the same read: an Alt-chord, one key press.
		if !utf8.FullRune(b[1:]) {
			return 0, false, true
		}
		_, size := utf8.DecodeRune(b[1:])
		return 1 + size, true, false
	}
}

// csiIsKey reports whether a CSI sequence (params+intermediates, final byte)
// is a key press. The terminal interleaves reports and events the user never
// pressed with the real keys:
//   - '?'/'>'/'='-prefixed params are responses to helix's startup queries
//     (device attributes, DECRPM mode reports, kitty-protocol probes)
//   - '<'-prefixed params are SGR mouse events (helix enables the mouse);
//     clicking and scrolling aren't keystrokes
//   - bare I/O finals are focus in/out events
//   - R/n/c/t/y finals are cursor-position, status, attribute and window
//     reports. Excluding R sacrifices modified-F3 (same encoding as a
//     cursor-position report, indistinguishable), which no helix motion uses;
//     plain F3 arrives as SS3 and still counts.
func csiIsKey(params []byte, final byte) bool {
	if len(params) > 0 {
		switch params[0] {
		case '?', '>', '=', '<':
			return false
		}
	}
	switch final {
	case 'R', 'n', 'c', 't', 'y':
		return false
	case 'I', 'O':
		return len(params) > 0
	}
	return true
}

// editTimer measures editing time. The clock starts on the first counted keystroke
// rather than at editor launch.
// Keystrokes that don't count toward the score don't start it either.
type editTimer struct {
	first time.Time // zero until the first counted keystroke
}

// keystroke marks a counted keystroke at time now; the first one starts the clock.
func (t *editTimer) keystroke(now time.Time) {
	if t.first.IsZero() {
		t.first = now
	}
}

// elapsed returns the editing time up to end, or zero when nothing was typed.
func (t *editTimer) elapsed(end time.Time) time.Duration {
	if t.first.IsZero() {
		return 0
	}
	return end.Sub(t.first)
}

// Result holds the outcome of a challenge attempt.
type Result struct {
	Output     string
	Keystrokes int
	Duration   time.Duration
}

// SplitPos is where the solution window is placed relative to the challenge.
// The string values are what gets persisted in the config file.
type SplitPos string

const (
	SplitOff    SplitPos = "off"
	SplitLeft   SplitPos = "left"
	SplitRight  SplitPos = "right"
	SplitTop    SplitPos = "top"
	SplitBottom SplitPos = "bottom"
)

// ParseSplitPos normalizes a stored config value; anything unrecognized is off.
func ParseSplitPos(s string) SplitPos {
	switch p := SplitPos(strings.ToLower(strings.TrimSpace(s))); p {
	case SplitLeft, SplitRight, SplitTop, SplitBottom:
		return p
	}
	return SplitOff
}

// Enabled reports whether the position actually opens a split.
func (p SplitPos) Enabled() bool {
	return p == SplitLeft || p == SplitRight || p == SplitTop || p == SplitBottom
}

// WindowMoves holds the key sequences injected to move helix's window focus in
// each direction.
type WindowMoves struct {
	Left, Right, Up, Down []byte
}

// DefaultWindowMoves returns helix's default window-mode moves (Ctrl-w + h/l/k/j).
func DefaultWindowMoves() WindowMoves {
	return WindowMoves{
		Left:  []byte("\x17h"),
		Right: []byte("\x17l"),
		Up:    []byte("\x17k"),
		Down:  []byte("\x17j"),
	}
}

// withDefaults fills any unset direction with the helix default, so a user can
// override only the directions they remapped.
func (m WindowMoves) withDefaults() WindowMoves {
	d := DefaultWindowMoves()
	if len(m.Left) == 0 {
		m.Left = d.Left
	}
	if len(m.Right) == 0 {
		m.Right = d.Right
	}
	if len(m.Up) == 0 {
		m.Up = d.Up
	}
	if len(m.Down) == 0 {
		m.Down = d.Down
	}
	return m
}

// splitArgs returns the helix CLI args and the focus-move sequence to inject for
// a challenge/solution split. Helix focuses the last file it opens, so when the
// solution goes left or top the challenge is passed last and starts focused with
// no injection. For right/bottom the solution must be last, so focus is moved
// back to the challenge (a left/up window move) once helix has rendered.
func splitArgs(pos SplitPos, challengePath, solutionPath string, mv WindowMoves) (args []string, focus []byte) {
	switch pos {
	case SplitLeft:
		return []string{"--vsplit", solutionPath, challengePath}, nil
	case SplitRight:
		return []string{"--vsplit", challengePath, solutionPath}, mv.Left
	case SplitTop:
		return []string{"--hsplit", solutionPath, challengePath}, nil
	case SplitBottom:
		return []string{"--hsplit", challengePath, solutionPath}, mv.Up
	}
	return []string{challengePath}, nil
}

// peekToggle is the default key (Ctrl-t) that flips a free look at the solution
// window in split mode.
const peekToggle = 0x14

// ParsePeekKey turns a config value like "ctrl-t" (also "ctrl+t", "C-t") into
// the control byte the input filter watches for.
func ParsePeekKey(s string) byte {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, prefix := range []string{"ctrl-", "ctrl+", "c-"} {
		if rest, ok := strings.CutPrefix(s, prefix); ok && len(rest) == 1 {
			c := rest[0]
			if c < 'a' || c > 'z' {
				break
			}
			b := c - 'a' + 1
			switch b {
			case '\t', '\n', '\r', 0x08: // ctrl-i, ctrl-j, ctrl-m, ctrl-h
				return peekToggle
			}
			return b
		}
	}
	return peekToggle
}

// PeekKeyName is the display name for a peek key byte, e.g. "ctrl-t".
func PeekKeyName(b byte) string {
	if b >= 1 && b <= 26 {
		return "ctrl-" + string(rune('a'+b-1))
	}
	return fmt.Sprintf("ctrl-0x%02x", b)
}

// escGap is the pause between an injected Esc and the window-move that follows.
// Sent in one write, helix would parse Esc+Ctrl-w as an Alt-chord: the Esc
// would not leave insert mode and the Ctrl-w would delete a word of the user's
// text. The gap makes helix see two separate key presses.
const escGap = 50 * time.Millisecond

// splitMoves returns the window-move sequences that put focus on the solution
// window and back on the challenge for the given layout. With only two windows
// directional moves are absolute (a move toward a window either lands there or
// stays put), so these also resync focus if it drifted, e.g. via a mouse click.
func splitMoves(pos SplitPos, mv WindowMoves) (toSolution, toChallenge []byte) {
	switch pos {
	case SplitLeft:
		return mv.Left, mv.Right
	case SplitRight:
		return mv.Right, mv.Left
	case SplitTop:
		return mv.Up, mv.Down
	case SplitBottom:
		return mv.Down, mv.Up
	}
	return nil, nil
}

// peekFilter mediates user input in split mode. Pressing the peek toggle moves
// focus into the read-only solution window "for free": neither the toggle nor
// any key pressed while peeking counts as a keystroke.
type peekFilter struct {
	peeking     bool
	toggle      byte   // the peek key's control byte (see ParsePeekKey)
	blocked     []byte // single bytes dropped while peeking
	toSolution  []byte
	toChallenge []byte
}

func newPeekFilter(pos SplitPos, toggle byte, mv WindowMoves) *peekFilter {
	toSol, toCh := splitMoves(pos, mv)
	blocked := []byte{':'}
	for _, seq := range [][]byte{mv.Left, mv.Right, mv.Up, mv.Down} {
		if len(seq) > 0 && !bytes.Contains(blocked, seq[:1]) {
			blocked = append(blocked, seq[0])
		}
	}
	return &peekFilter{toggle: toggle, blocked: blocked, toSolution: toSol, toChallenge: toCh}
}

// process consumes one stdin read and returns the chunks to forward to the pty
// and the bytes that count toward the keystroke score (those typed while not
// peeking; the toggle and the injected moves are free). The writer must pause
// escGap between chunks: a chunk boundary is only emitted after an injected
// Esc, which must not be parsed as an Alt-chord with the bytes that follow.
func (f *peekFilter) process(in []byte) (outs [][]byte, counted []byte) {
	cur := []byte{}
	for _, b := range in {
		switch {
		case b == f.toggle:
			move := f.toSolution
			if f.peeking {
				move = f.toChallenge
			}
			f.peeking = !f.peeking
			// Esc first so a toggle pressed in insert mode (or half-typed
			// command) safely lands in normal mode before the window move. The
			// move itself is chunked too in case a custom one contains an Esc.
			outs = append(outs, append(cur, 0x1b))
			cur = []byte{}
			if chunks := escChunks(move); len(chunks) > 0 {
				outs = append(outs, chunks[:len(chunks)-1]...)
				cur = append(cur, chunks[len(chunks)-1]...)
			}
		case f.peeking && bytes.Contains(f.blocked, []byte{b}):
			// dropped while peeking
		default:
			cur = append(cur, b)
			if !f.peeking {
				counted = append(counted, b)
			}
		}
	}
	outs = append(outs, cur)
	return outs, counted
}

// SplitOptions configures how the solution split behaves.
type SplitOptions struct {
	Pos     SplitPos
	PeekKey byte          // control byte toggling the free solution peek; 0 = default
	Moves   WindowMoves   // window focus moves; unset directions use helix defaults
	Quits   []QuitCommand // extra quit detections, checked before the builtins
	// SolutionKey names the stable per-solution directory the solution file is
	// written to (typically the challenge ID). Empty or unsafe values fall back
	// to a hash of the solution content.
	SolutionKey string
}

func modFile(module string) string {
	return "module " + module + "\n\ngo 1.21\n"
}

// ensureModFile writes dir/go.mod when it is missing or stale.
func ensureModFile(dir, module string) error {
	path := filepath.Join(dir, "go.mod")
	content := modFile(module)
	if b, err := os.ReadFile(path); err == nil && string(b) == content {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// goWorkFile is the go.work written into the challenge directory. Together
// with launching helix with `-w <challenge dir>`, it pins gopls's workspace to
// our own modules.
func goWorkFile(extraDirs ...string) string {
	var b strings.Builder
	b.WriteString("go 1.21\n\nuse (\n\t.\n")
	for _, d := range extraDirs {
		b.WriteString("\t")
		if strings.ContainsAny(d, " \t\"'`") {
			fmt.Fprintf(&b, "%q", d)
		} else {
			b.WriteString(d)
		}
		b.WriteString("\n")
	}
	b.WriteString(")\n")
	return b.String()
}

// ensureSolutionFile writes the read-only solution into a stable directory of
// its own and returns the file path.
func ensureSolutionFile(key, content, fallbackDir string) (string, error) {
	dir := ""
	if cache, err := os.UserCacheDir(); err == nil {
		dir = filepath.Join(cache, "gohelix", "solutions", solutionKey(key, content))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			dir = ""
		}
	}
	if dir == "" {
		dir = filepath.Join(fallbackDir, "solution")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	// Before the content fast-path below, so dirs cached by older gohelix
	// versions (no go.mod yet) are upgraded too.
	if err := ensureModFile(dir, "gohelix.solution"); err != nil {
		return "", err
	}

	path := filepath.Join(dir, "solution.go")
	if b, err := os.ReadFile(path); err == nil && string(b) == content {
		return path, nil // reused from a previous run
	}
	// The file is 0444, so a plain overwrite would fail: remove first (the dir
	// itself is writable).
	_ = os.Remove(path)
	if err := os.WriteFile(path, []byte(content), 0o444); err != nil {
		// Possibly lost a write race with a second gohelix instance; if that
		// instance wrote the same content, the file is still correct.
		if b, rerr := os.ReadFile(path); rerr == nil && string(b) == content {
			return path, nil
		}
		return "", err
	}
	return path, nil
}

// solutionKey returns a filesystem-safe directory name for a solution.
func solutionKey(key, content string) string {
	safe := key != ""
	for _, r := range key {
		if r != '_' && r != '-' && r != '.' && !('a' <= r && r <= 'z') &&
			!('A' <= r && r <= 'Z') && !('0' <= r && r <= '9') {
			safe = false
			break
		}
	}
	if safe {
		return key
	}
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:8])
}

// Runner implements tea.ExecCommand to launch helix in a PTY,
// proxy I/O, and count keystrokes.
type Runner struct {
	content  string
	solution string // when non-empty, opened read-only in a split per opts
	opts     SplitOptions
	result   Result
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
}

// New creates a runner with the given file content.
func New(content string) *Runner {
	return &Runner{content: content}
}

// NewWithSolution creates a runner that also opens solution read-only in a
// split, for studying the target while editing.
func NewWithSolution(content, solution string, opts SplitOptions) *Runner {
	if opts.PeekKey == 0 {
		opts.PeekKey = peekToggle
	}
	opts.Moves = opts.Moves.withDefaults()
	return &Runner{content: content, solution: solution, opts: opts}
}

func (r *Runner) SetStdin(in io.Reader)   { r.stdin = in }
func (r *Runner) SetStdout(out io.Writer) { r.stdout = out }
func (r *Runner) SetStderr(err io.Writer) { r.stderr = err }

// GetResult returns the result after Run() completes.
func (r *Runner) GetResult() Result { return r.result }

// Run launches helix on a temp file, proxies I/O through a PTY,
// counts keystrokes, and captures the edited file contents.
func (r *Runner) Run() error {
	// Create an isolated temp directory so the LSP doesn't pick up stray .go files.
	tmpDir, err := os.MkdirTemp("", "gohelix-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := tmpDir + "/challenge.go"
	if err := os.WriteFile(tmpPath, []byte(r.content), 0o644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := ensureModFile(tmpDir, "gohelix.challenge"); err != nil {
		return fmt.Errorf("writing go.mod: %w", err)
	}

	// In split mode, write the solution into its own directory and pass both
	// files to helix. The solution file is written read-only (0o444) so helix
	// refuses to save it; helix has no true read-only buffer, so this is the
	// closest guard available.
	splitting := r.solution != "" && r.opts.Pos.Enabled()
	args := []string{tmpPath}
	var focus []byte
	var workDirs []string
	if splitting {
		solPath, err := ensureSolutionFile(r.opts.SolutionKey, r.solution, tmpDir)
		if err != nil {
			return fmt.Errorf("writing solution file: %w", err)
		}
		workDirs = append(workDirs, filepath.Dir(solPath))
		args, focus = splitArgs(r.opts.Pos, tmpPath, solPath, r.opts.Moves)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "go.work"), []byte(goWorkFile(workDirs...)), 0o644); err != nil {
		return fmt.Errorf("writing go.work: %w", err)
	}

	// Verify helix is installed
	hxPath, err := exec.LookPath("hx")
	if err != nil {
		return fmt.Errorf("helix (hx) not found in PATH, install it from https://helix-editor.com")
	}

	// Start helix in a PTY, with its working directory pinned to the challenge
	// dir so the go.work there defines the LSP workspace.
	ptmx, err := pty.New()
	if err != nil {
		return fmt.Errorf("opening pty: %w", err)
	}
	cmd := ptmx.Command(hxPath, append([]string{"-w", tmpDir}, args...)...)
	cmd.Env = append(os.Environ(), "COLORTERM=truecolor")
	if err := cmd.Start(); err != nil {
		ptmx.Close()
		return fmt.Errorf("starting helix: %w", err)
	}

	// Keep the PTY's window size in sync with the real terminal.
	// This is platform-specific (SIGWINCH on Unix, console events on Windows).
	stopResize := watchResize(ptmx)

	// Set terminal to raw mode for proper keystroke forwarding
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		ptmx.Close()
		return fmt.Errorf("setting raw mode: %w", err)
	}

	var keystrokes atomic.Int64
	var timer editTimer // written by the input goroutine, read after inputDone

	// Proxy PTY output → terminal stdout. firstOutput is closed on helix's first
	// render so split mode can move focus once helix is actually accepting input
	// (bytes written before then are discarded during startup).
	firstOutput := make(chan struct{})
	go func() {
		signaled := false
		buf := make([]byte, 32*1024)
		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				if !signaled {
					signaled = true
					close(firstOutput)
				}
				if _, writeErr := os.Stdout.Write(buf[:n]); writeErr != nil {
					return
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	// For right/bottom splits helix opens with the solution (last window) focused.
	// Once it has rendered, move focus back to the challenge so the user starts
	// editing the right file. This is injected by us, not read from stdin, so it
	// isn't counted as a keystroke.
	if len(focus) > 0 {
		go func() {
			select {
			case <-firstOutput:
			case <-time.After(2 * time.Second): // fall back if helix never renders
			}
			time.Sleep(150 * time.Millisecond) // let the initial draw settle
			for _, chunk := range escChunks(focus) {
				if _, err := ptmx.Write(chunk); err != nil {
					return
				}
				time.Sleep(escGap)
			}
		}()
	}

	// Proxy terminal stdin → PTY input, counting the key presses it carries.
	// A cancelable reader lets us stop this goroutine the instant helix exits.
	stdin, err := cancelreader.NewReader(os.Stdin)
	if err != nil {
		_ = term.Restore(fd, oldState)
		ptmx.Close()
		return fmt.Errorf("creating input reader: %w", err)
	}

	inputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		buf := make([]byte, 256)
		var keys keyCounter
		var filter *peekFilter
		var quits *quitDetector
		if splitting {
			filter = newPeekFilter(r.opts.Pos, r.opts.PeekKey, r.opts.Moves)
			toSolution, toChallenge := splitMoves(r.opts.Pos, r.opts.Moves)
			quits = newQuitDetector(r.opts.Quits, toSolution, toChallenge)
		}
		for {
			n, readErr := stdin.Read(buf)
			if n > 0 {
				outs, countable := [][]byte{buf[:n]}, buf[:n]
				if filter != nil {
					outs, countable = filter.process(buf[:n])
				}
				if events := keys.count(countable); events > 0 {
					keystrokes.Add(int64(events))
					timer.keystroke(time.Now())
				}
				for i, out := range outs {
					if i > 0 {
						time.Sleep(escGap) // keep injected Esc and move as separate key presses
					}
					if len(out) == 0 {
						continue
					}
					if _, writeErr := ptmx.Write(out); writeErr != nil {
						return
					}
					// In split mode, a bare :wq/:q leaves the read-only solution
					// window open and helix running. Detect the user's quit and
					// inject the quit-all variant so the whole editor exits.
					if quits != nil {
						for _, chunk := range quits.inject(out) {
							if _, writeErr := ptmx.Write(chunk); writeErr != nil {
								return
							}
							time.Sleep(escGap) // separate Esc from what follows
						}
					}
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	// Wait for helix to exit. The editing clock ends here, before our own
	// cleanup below.
	_ = cmd.Wait()
	end := time.Now()

	// Stop the input proxy and wait for it to exit before returning, so it can't
	// consume the next keystroke meant for the TUI.
	stdin.Cancel()
	<-inputDone

	// Cleanup
	stopResize()
	_ = term.Restore(fd, oldState)
	ptmx.Close()

	// Read the edited file
	output, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("reading result: %w", err)
	}

	r.result = Result{
		Output:     string(output),
		Keystrokes: int(keystrokes.Load()),
		Duration:   timer.elapsed(end),
	}
	return nil
}
