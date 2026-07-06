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

	cols, rows := 80, 24
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		cols, rows = w, h
	}
	d := newDispatcher(os.Stdout, cols, rows)

	// Forward terminal resizes to the child's PTY and the termstate engine
	// (initial + on SIGWINCH).
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			_ = pty.InheritSize(os.Stdin, ptmx)
			if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
				d.Resize(w, h)
			}
		}
	}()
	ch <- syscall.SIGWINCH
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
