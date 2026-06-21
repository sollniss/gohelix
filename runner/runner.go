package runner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"
)

// Result holds the outcome of a challenge attempt.
type Result struct {
	Output     string
	Keystrokes int
	Duration   time.Duration
}

// Runner implements tea.ExecCommand to launch helix in a PTY,
// proxy I/O, and count keystrokes.
type Runner struct {
	content string
	result  Result
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
}

// New creates a runner with the given file content.
func New(content string) *Runner {
	return &Runner{content: content}
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

	// Verify helix is installed
	hxPath, err := exec.LookPath("hx")
	if err != nil {
		return fmt.Errorf("helix (hx) not found in PATH, install it from https://helix-editor.com")
	}

	// Start helix in a PTY
	cmd := exec.Command(hxPath, tmpPath)
	cmd.Env = append(os.Environ(), "COLORTERM=truecolor")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("starting helix: %w", err)
	}

	// Handle terminal resize signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	go func() {
		for range sigCh {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	_ = pty.InheritSize(os.Stdin, ptmx) // initial size

	// Set terminal to raw mode for proper keystroke forwarding
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		ptmx.Close()
		return fmt.Errorf("setting raw mode: %w", err)
	}

	start := time.Now()
	var keystrokes atomic.Int64

	// Proxy PTY output → terminal stdout
	go func() {
		_, _ = io.Copy(os.Stdout, ptmx)
	}()

	// Proxy terminal stdin → PTY input, counting each read as a keystroke.
	// A cancelable reader lets us stop this goroutine the instant helix exits;
	// otherwise it lingers blocked on Read and swallows the first keystroke
	// once control returns to the TUI (e.g. the first j on the result screen).
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
		for {
			n, readErr := stdin.Read(buf)
			if n > 0 {
				keystrokes.Add(1)
				if _, writeErr := ptmx.Write(buf[:n]); writeErr != nil {
					return
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	// Wait for helix to exit
	_ = cmd.Wait()
	duration := time.Since(start)

	// Stop the input proxy and wait for it to exit before returning, so it can't
	// consume the next keystroke meant for the TUI.
	stdin.Cancel()
	<-inputDone

	// Cleanup
	signal.Stop(sigCh)
	close(sigCh)
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
		Duration:   duration,
	}
	return nil
}
