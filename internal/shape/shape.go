package shape

import (
	"strings"

	"github.com/benoitkugler/textprocessing/fribidi"
)

// Shape reorders and Arabic-shapes s for a terminal with no bidi support.
// Base direction is auto-detected per line from the first strong character
// (fribidi.ON), so pure-LTR lines pass through unchanged.
//
// fribidi's one-shot transform assumes a single line, so we split on '\n'
// and shape each line independently.
func Shape(s string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = shapeLine(line)
	}
	return strings.Join(lines, "\n")
}

func shapeLine(line string) string {
	if line == "" {
		return line
	}
	base := fribidi.ParType(fribidi.ON) // auto-detect base direction
	vis, _ := fribidi.LogicalToVisual(fribidi.DefaultFlags, []rune(line), &base)
	return string(vis.Str)
}
