// Package difft is a thin integration with difftastic (https://difftastic.wilfred.me.uk).
// It runs difft in JSON mode and reduces the output to a neutral structure a
// side-by-side renderer can consume: a line alignment plus the changed character
// spans on each side. It has no UI dependencies.
package difft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Available reports whether the difft executable is on PATH.
func Available() bool {
	_, err := exec.LookPath("difft")
	return err == nil
}

// Range is a changed character span within a line, in source-rune offsets.
type Range struct{ Start, End int }

// Result is difftastic's structural diff reduced to what a side-by-side renderer
// needs.
type Result struct {
	// Rows pairs before/after line indices (0-based). A value of -1 means that
	// side has no line for that row (an insertion or deletion).
	Rows [][2]int
	// Lhs and Rhs map a line index to the changed character spans on that line.
	// When a file fails to parse, difft falls back to a line-based diff and these
	// spans cover whole changed lines; the alignment (Rows) is good either way.
	Lhs map[int][]Range
	Rhs map[int][]Range
}

// Align runs difftastic in JSON mode on left vs right and returns the parsed
// alignment. tool is the difft executable name or path. It returns an error if
// difft can't run or its output can't be parsed, so callers can fall back.
func Align(tool, left, right string) (Result, error) {
	dir, leftPath, rightPath, err := writeTemp(left, right)
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(dir)

	cmd := exec.Command(tool, "--display", "json", leftPath, rightPath)
	cmd.Env = append(os.Environ(), "DFT_UNSTABLE=yes")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	// difft exits non-zero when files differ, so a run error is expected; we rely
	// on successful parsing instead of the exit code.
	_ = cmd.Run()

	res, err := parse(out.String())
	if err != nil {
		return res, err
	}
	// difft emits a final virtual aligned line for the files' trailing newline.
	// It references no real source line, so without trimming it the diff renders
	// an empty row at the bottom. Drop such trailing rows.
	res.Rows = trimPhantomRows(res.Rows, lineCount(left), lineCount(right))
	return res, nil
}

// lineCount returns the number of content lines in s, ignoring a single trailing
// newline (so it matches how the diff renderer splits the highlighted source).
func lineCount(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// trimPhantomRows drops trailing aligned rows that reference no real line on
// either side.
func trimPhantomRows(rows [][2]int, leftN, rightN int) [][2]int {
	for len(rows) > 0 {
		r := rows[len(rows)-1]
		leftOK := r[0] >= 0 && r[0] < leftN
		rightOK := r[1] >= 0 && r[1] < rightN
		if leftOK || rightOK {
			break
		}
		rows = rows[:len(rows)-1]
	}
	return rows
}

// writeTemp writes left/right to a fresh temp dir as before.go/after.go so difft
// can read them. The caller must os.RemoveAll(dir).
func writeTemp(left, right string) (dir, leftPath, rightPath string, err error) {
	dir, err = os.MkdirTemp("", "hx-difft-")
	if err != nil {
		return
	}
	leftPath = filepath.Join(dir, "before.go")
	rightPath = filepath.Join(dir, "after.go")
	if err = os.WriteFile(leftPath, []byte(left), 0o644); err != nil {
		return
	}
	err = os.WriteFile(rightPath, []byte(right), 0o644)
	return
}

// parse converts difft's JSON output into a Result.
func parse(jsonOut string) (Result, error) {
	type change struct {
		Start int `json:"start"`
		End   int `json:"end"`
	}
	type side struct {
		LineNumber int      `json:"line_number"`
		Changes    []change `json:"changes"`
	}
	type item struct {
		Lhs *side `json:"lhs"`
		Rhs *side `json:"rhs"`
	}
	type file struct {
		AlignedLines [][]*int `json:"aligned_lines"`
		Chunks       [][]item `json:"chunks"`
	}

	jsonOut = strings.TrimSpace(jsonOut)
	if jsonOut == "" {
		return Result{}, fmt.Errorf("difft: no output")
	}

	// difft emits a single object for a two-file diff, or an array of objects in
	// multi-file/git mode. Accept either.
	var f file
	if strings.HasPrefix(jsonOut, "[") {
		var fs []file
		if err := json.Unmarshal([]byte(jsonOut), &fs); err != nil {
			return Result{}, err
		}
		if len(fs) == 0 {
			return Result{}, fmt.Errorf("difft: empty file array")
		}
		f = fs[0]
	} else if err := json.Unmarshal([]byte(jsonOut), &f); err != nil {
		return Result{}, err
	}

	res := Result{
		Rows: make([][2]int, 0, len(f.AlignedLines)),
		Lhs:  map[int][]Range{},
		Rhs:  map[int][]Range{},
	}
	for _, p := range f.AlignedLines {
		row := [2]int{-1, -1}
		if len(p) >= 1 && p[0] != nil {
			row[0] = *p[0]
		}
		if len(p) >= 2 && p[1] != nil {
			row[1] = *p[1]
		}
		res.Rows = append(res.Rows, row)
	}

	collect := func(m map[int][]Range, s *side) {
		if s == nil {
			return
		}
		for _, c := range s.Changes {
			if c.End > c.Start {
				m[s.LineNumber] = append(m[s.LineNumber], Range(c))
			}
		}
	}
	for _, chunk := range f.Chunks {
		for _, it := range chunk {
			collect(res.Lhs, it.Lhs)
			collect(res.Rhs, it.Rhs)
		}
	}
	return res, nil
}
