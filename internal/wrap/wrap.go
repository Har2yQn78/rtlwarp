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

	// Forward terminal resizes to the child's PTY (initial + on SIGWINCH).
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			_ = pty.InheritSize(os.Stdin, ptmx)
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

	p := newPipe(os.Stdout)
	_, _ = io.Copy(p, ptmx) // returns when the child closes the PTY
	_ = p.Close()

	return c.Wait()
}
