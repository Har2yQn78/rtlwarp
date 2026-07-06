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
	vis, _ := ShapeRunes([]rune(line))
	// fribidi inserts zero-width U+FEFF fillers where a lam-alef ligature
	// absorbed a character. They add a visible cell gap in some terminals and
	// carry no meaning for display, so strip them here (Shape doesn't need the
	// position map; ShapeRunes keeps the fillers for callers that do).
	return strings.ReplaceAll(string(vis), "\ufeff", "")
}

func ShapeRunes(logical []rune) (visual []rune, visualToLogical []int) {
	if len(logical) == 0 {
		return nil, nil
	}
	base := fribidi.ParType(fribidi.ON) // auto-detect base direction
	vis, _ := fribidi.LogicalToVisual(fribidi.DefaultFlags, logical, &base)
	return vis.Str, vis.VisualToLogical
}
