package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"rtlwrap/internal/wrap"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: rtlwrap <command> [args...]")
		os.Exit(2)
	}
	err := wrap.Run(os.Args[1:])
	if err == nil {
		return
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.ExitCode()) // propagate the child's exit code
	}
	fmt.Fprintln(os.Stderr, "rtlwrap:", err)
	os.Exit(1)
}
