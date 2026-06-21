package ui

import (
	"math/rand/v2"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sollniss/gohelix/challenges"
	"github.com/sollniss/gohelix/difft"
	"github.com/sollniss/gohelix/runner"
	"github.com/sollniss/gohelix/score"
)

type state int

const (
	stateMenu    state = iota // challenge picker (optional, accessed via Esc from preview)
	statePreview              // before/after preview (default start state)
	stateResult               // post-challenge result
)

// Messages
type challengeCompleteMsg struct {
	result runner.Result
}

type challengeErrMsg struct {
	err error
}

// requestDiffMsg asks the model to (re)compute the external diff for the
// current context. Emitted from Init so the request runs against the real
// model (Init's receiver is a copy, so it can't mutate diffSeq itself).
type requestDiffMsg struct{}

// diffReadyMsg carries the parsed difft alignment. seq guards against stale
// results: if the context changed while difft was running, the result is
// discarded. aligned is nil when difft failed, so the renderer falls back to
// the bat side-by-side view.
type diffReadyMsg struct {
	seq     int
	aligned *difft.Result
}

// DiffMode controls how diffs are displayed.
type DiffMode int

const (
	DiffBuiltin DiffMode = iota // bat side-by-side, rendered in the TUI (default)
	DiffDifft                   // difft-aligned 50/50 columns (when difft is on PATH)
)

// Model is the main bubbletea model.
type Model struct {
	state      state
	challenges []challenges.Challenge
	scores     *score.Store
	width      int
	height     int

	// Permutation-based ordering: perm[permIdx] gives the current challenge index.
	perm    []int
	permIdx int

	// Menu cursor and scroll offset (separate from permutation flow)
	menuCursor int
	menuScroll int

	showHint       bool
	previewScroll  int // vertical scroll offset for the preview code area
	previewHScroll int // horizontal scroll offset for the preview code area
	resultScroll   int // vertical scroll offset for the result diff
	resultHScroll  int // horizontal scroll offset for the result diff

	// Diff display. bat side-by-side is the default; difft, when available on
	// PATH, is an optional enhancement that aligns the columns and highlights
	// changes (toggled with "d"). gohelix runs difft in JSON mode and lays the
	// result out itself into exact 50/50 columns.
	diffMode       DiffMode     // current active mode
	difftAvailable bool         // whether difft is on PATH
	difftResult    difft.Result // parsed difft alignment for the current context
	diffSeq        int          // request counter; latest-wins guard for async results

	// Scroll anchor for diff-mode toggles. The two modes lay lines out
	// differently (difft inserts gap rows), so a toggle preserves the left
	// ("before"/"your solution") source line at the top of the viewport rather
	// than the raw offset. When switching to difft the alignment loads
	// asynchronously, so the anchor is applied once diffReadyMsg with the
	// matching seq arrives.
	pendingScrollLine int // left source line to pin to the top
	pendingScrollSeq  int // diffSeq this anchor waits on; 0 means none

	lastResult *runner.Result
	passed     bool
	isRecord   bool
	errMsg     string
}

// New creates the UI model, shuffling challenges into a random permutation.
func New(chs []challenges.Challenge) Model {
	return newModel(chs, rand.Perm(len(chs)))
}

// NewAt creates the UI model starting at a specific challenge index.
func NewAt(chs []challenges.Challenge, idx int) Model {
	return newModel(chs, permStartingAt(len(chs), idx))
}

// permStartingAt returns a fresh random permutation of [0,n) whose first element
// is start. Each challenge appears exactly once, so cycling through it never
// repeats one until every other has been seen.
func permStartingAt(n, start int) []int {
	perm := rand.Perm(n)
	for i, v := range perm {
		if v == start {
			perm[0], perm[i] = perm[i], perm[0]
			break
		}
	}
	return perm
}

func newModel(chs []challenges.Challenge, perm []int) Model {
	return Model{
		challenges:     chs,
		scores:         score.NewStore(),
		state:          statePreview,
		perm:           perm,
		permIdx:        0,
		diffMode:       DiffBuiltin, // bat side-by-side by default; "d" enables difft
		difftAvailable: difft.Available(),
	}
}

// current returns the challenge at the current permutation position.
func (m Model) current() challenges.Challenge {
	return m.challenges[m.perm[m.permIdx]]
}

// toggleDiffMode switches between the bat side-by-side view and the difft-aligned
// view, returning a command to compute the difft diff when switching to it.
// It is a no-op when difft is not available.
func (m *Model) toggleDiffMode() tea.Cmd {
	if !m.difftAvailable {
		return nil
	}
	if m.diffMode == DiffBuiltin {
		m.diffMode = DiffDifft
		return m.requestDiff()
	}
	m.diffMode = DiffBuiltin
	return nil
}

// requestDiff returns a command that computes the difft alignment for the current
// context off the UI goroutine. It bumps diffSeq so that only the most recent
// request's result is applied. Returns nil when difft isn't active.
func (m *Model) requestDiff() tea.Cmd {
	if m.diffMode != DiffDifft || !m.difftAvailable {
		return nil
	}
	// Drop the previous alignment immediately. The diff runs asynchronously, and
	// the context (preview vs result, which challenge) may have just changed; the
	// renderer falls back to the bat layout until the fresh result arrives, rather
	// than applying a stale alignment to different content.
	m.difftResult = difft.Result{}
	m.diffSeq++
	seq := m.diffSeq

	ch := m.current()
	var left, right string
	if m.state == stateResult && m.lastResult != nil {
		left, right = m.lastResult.Output, ch.After
	} else {
		left, right = ch.Before, ch.After
	}

	return func() tea.Msg {
		res, err := difft.Align("difft", left, right)
		if err != nil {
			return diffReadyMsg{seq: seq} // empty -> renderer falls back to bat view
		}
		return diffReadyMsg{seq: seq, aligned: &res}
	}
}

// reshuffle generates a new random permutation and resets the index.
func (m *Model) reshuffle() {
	m.perm = rand.Perm(len(m.challenges))
	m.permIdx = 0
}

// advance moves to the next challenge in the permutation, reshuffling if exhausted.
func (m *Model) advance() {
	m.permIdx++
	if m.permIdx >= len(m.perm) {
		m.reshuffle()
	}
	m.showHint = false
	m.previewScroll = 0
	m.previewHScroll = 0
}
