package wrap

import (
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// Run spawns argv on a PTY, forwarding the user's keystrokes unchanged and
// shaping the child's output through the pipe. It blocks until the child
// exits and returns the child's error (an *exec.ExitError carries its code).
func Run(argv []string) error {
	c := exec.Command(argv[0], argv[1:]...)
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	defer ptmx.Close()

	d := newDispatcher(os.Stdout, 80, 24)

	// Forward terminal resizes to the child's PTY, then size the engine from the
	// PTY, not from os.Stdout: the PTY winsize is what the child actually draws
	// into, so the engine's grid must match it exactly (an embedded terminal,
	// e.g. Zed, can report a different size for stdout than the PTY the child
	// sees). Run once synchronously so both are sized before output flows, then
	// on every SIGWINCH.
	resize := func() {
		_ = pty.InheritSize(os.Stdin, ptmx)
		if rows, cols, err := pty.Getsize(ptmx); err == nil {
			d.Resize(cols, rows)
		}
	}
	resize()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			resize()
		}
	}()
	defer signal.Stop(ch)

	// Put the real terminal in raw mode so keystrokes reach the child verbatim.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		old, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return err
		}
		defer term.Restore(int(os.Stdin.Fd()), old)
	}

	// Keystrokes are sent as-is (the child expects logical order); only the
	// child's output is shaped.
	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()

	_, _ = io.Copy(d, ptmx) // returns when the child closes the PTY
	_ = d.Close()

	return c.Wait()
}
