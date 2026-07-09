//go:build windows

package runner

import (
	"time"

	pty "github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/windows"
)

// resizePollInterval is how often the Windows watcher checks the console for a
// size change. Windows has no SIGWINCH, and the console's WINDOW_BUFFER_SIZE
// events share the single input buffer that the keystroke-forwarding goroutine
// already drains, so reading them here would steal the user's input. Polling
// the output handle's screen-buffer info sidesteps that buffer entirely.
const resizePollInterval = 250 * time.Millisecond

// watchResize keeps the PTY's window size in sync with the real terminal by
// polling the console screen buffer, and applies the initial size once. The
// returned stop function tears the watcher down and must be called during
// cleanup.
func watchResize(ptmx pty.Pty) (stop func()) {
	done := make(chan struct{})
	w, h := syncSize(ptmx, 0, 0) // initial size
	go func() {
		ticker := time.NewTicker(resizePollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				w, h = syncSize(ptmx, w, h)
			}
		}
	}()
	var closed bool
	return func() {
		if !closed {
			closed = true
			close(done)
		}
	}
}

// syncSize reads the console's visible window size and, if it differs from the
// last-applied (lastW, lastH), pushes it into the PTY. It returns the size it
// observed so the caller can track changes. Errors are ignored; a failed
// resize is not worth aborting the session over.
func syncSize(ptmx pty.Pty, lastW, lastH int) (w, h int) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Stdout, &info); err != nil {
		return lastW, lastH
	}
	w = int(info.Window.Right-info.Window.Left) + 1
	h = int(info.Window.Bottom-info.Window.Top) + 1
	if w <= 0 || h <= 0 || (w == lastW && h == lastH) {
		return lastW, lastH
	}
	_ = ptmx.Resize(w, h)
	return w, h
}
