//go:build !windows

package runner

import (
	"os"
	"os/signal"
	"syscall"

	pty "github.com/aymanbagabas/go-pty"
	"golang.org/x/term"
)

// watchResize keeps the PTY's window size in sync with the real terminal by
// listening for SIGWINCH, and applies the initial size once. The returned stop
// function tears the watcher down and must be called during cleanup.
func watchResize(ptmx pty.Pty) (stop func()) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	go func() {
		for range sigCh {
			syncSize(ptmx)
		}
	}()
	syncSize(ptmx) // initial size
	return func() {
		signal.Stop(sigCh)
		close(sigCh)
	}
}

// syncSize pushes the current terminal size into the PTY, ignoring errors (a
// failed resize is not worth aborting the session over).
func syncSize(ptmx pty.Pty) {
	w, h, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return
	}
	_ = ptmx.Resize(w, h)
}
