package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/Har2yQn78/rtlwrap/internal/wrap"
)

// version is set at build time via -ldflags "-X main.version=..." (GoReleaser).
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: rtlwrap <command> [args...]")
		os.Exit(2)
	}
	if os.Args[1] == "--version" || os.Args[1] == "-v" {
		fmt.Println("rtlwrap", version)
		return
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
