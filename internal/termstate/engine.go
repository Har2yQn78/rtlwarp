package termstate

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hinshun/vt10x"

	"github.com/Har2yQn78/rtlwrap/internal/shape"
)

// vt10x attribute bits (mirrors the unexported consts in vt10x/state.go).
const (
	attrReverse   = 1 << 0
	attrUnderline = 1 << 1
	attrBold      = 1 << 2
	attrItalic    = 1 << 4
	attrBlink     = 1 << 5
)

// Engine feeds child output into a vt10x virtual terminal and renders the grid
// to w, reshaping RTL rows. Not safe for concurrent Write; drive it from one
// goroutine (the output copy loop).
type Engine struct {
	vt         vt10x.Terminal
	w          io.Writer
	cols, rows int
	prev       []string // last-emitted content per row, for diffing
}

// New returns an Engine rendering a cols×rows virtual terminal to w.
func New(w io.Writer, cols, rows int) *Engine {
	return &Engine{
		vt:   vt10x.New(vt10x.WithSize(cols, rows)),
		w:    w,
		cols: cols,
		rows: rows,
		prev: make([]string, rows),
	}
}

// Resize resizes the virtual terminal and forces a full repaint next render.
func (e *Engine) Resize(cols, rows int) {
	e.vt.Resize(cols, rows)
	e.cols, e.rows = cols, rows
	e.prev = make([]string, rows) // invalidate: "" matches no rendered row
}

// Write feeds raw child bytes to the virtual terminal, then renders. It always
// reports len(p) consumed (vt10x parses the whole buffer).
func (e *Engine) Write(p []byte) (int, error) {
	if _, err := e.vt.Write(p); err != nil {
		return 0, err
	}
	return len(p), e.render()
}

func (e *Engine) render() error {
	e.vt.Lock()
	defer e.vt.Unlock()

	cols, rows := e.vt.Size()
	if cols != e.cols || rows != e.rows || len(e.prev) != rows {
		e.cols, e.rows, e.prev = cols, rows, make([]string, rows)
	}

	var buf bytes.Buffer
	// Hide cursor + disable autowrap during the repaint so a full-width row's
	// last column can't scroll the screen. Restored at the end.
	buf.WriteString("\x1b[?25l\x1b[?7l")

	l2v := make([][]int, rows) // per-row visualToLogical, kept for cursor mapping
	for y := 0; y < rows; y++ {
		line, v2l := e.renderRow(y, cols)
		l2v[y] = v2l
		if line == e.prev[y] {
			continue
		}
		e.prev[y] = line
		fmt.Fprintf(&buf, "\x1b[%d;1H\x1b[2K", y+1) // move to row start, clear it
		buf.WriteString(line)
	}

	buf.WriteString("\x1b[?7h") // re-enable autowrap
	cur := e.vt.Cursor()
	cx := cur.X
	if cur.Y >= 0 && cur.Y < rows {
		cx = visualX(l2v[cur.Y], cur.X, cols)
	}
	fmt.Fprintf(&buf, "\x1b[%d;%dH", cur.Y+1, cx+1)
	if e.vt.CursorVisible() {
		buf.WriteString("\x1b[?25h")
	}

	_, err := e.w.Write(buf.Bytes())
	return err
}

// renderRow builds the SGR-encoded visual string for grid row y and returns it
// with that row's visualToLogical map (nil if the row had no runes).
//
// ponytail: emits the full cols width (trailing spaces included) rather than
// trimming. The row is cleared with \x1b[2K first, so trailing default cells are
// harmless, and emitting them keeps any full-row background color correct. Trim
// only if the extra bytes on wide terminals ever measure.
func (e *Engine) renderRow(y, cols int) (string, []int) {
	logical := make([]rune, cols)
	type attr struct {
		fg, bg vt10x.Color
		mode   int16
	}
	attrs := make([]attr, cols)
	for x := 0; x < cols; x++ {
		g := e.vt.Cell(x, y)
		r := g.Char
		if r == 0 {
			r = ' '
		}
		logical[x] = r
		attrs[x] = attr{g.FG, g.BG, g.Mode}
	}

	vis, v2l := shape.ShapeRunes(logical)

	var sb strings.Builder
	last := attr{fg: ^vt10x.Color(0)} // impossible value forces first SGR emit
	for i, r := range vis {
		if r == '\ufeff' { // lam-alef filler: no cell, keeps the map 1:1
			continue
		}
		a := attrs[v2l[i]]
		if a != last {
			sb.WriteString(sgr(a.fg, a.bg, a.mode))
			last = a
		}
		sb.WriteRune(r)
	}
	sb.WriteString("\x1b[0m")
	return sb.String(), v2l
}

// visualX maps a logical column to its visual column in a reshaped row.
//
// ponytail: does not subtract stripped U+FEFF fillers to the cursor's left, so
// in a row containing lam-alef ligatures the cursor can sit one cell off per
// ligature. Rare; fix by counting skipped fillers when it actually bites.
func visualX(v2l []int, logicalX, cols int) int {
	for i, li := range v2l {
		if li == logicalX {
			return i
		}
	}
	if logicalX >= cols {
		return cols - 1
	}
	return logicalX
}

// sgr renders a full SGR sequence (leading reset, then attributes and colors)
// for the given cell. Resetting first each time is a few extra bytes but avoids
// tracking incremental attribute add/remove state.
func sgr(fg, bg vt10x.Color, mode int16) string {
	p := []string{"0"}
	if mode&attrBold != 0 {
		p = append(p, "1")
	}
	if mode&attrItalic != 0 {
		p = append(p, "3")
	}
	if mode&attrUnderline != 0 {
		p = append(p, "4")
	}
	if mode&attrBlink != 0 {
		p = append(p, "5")
	}
	if mode&attrReverse != 0 {
		p = append(p, "7")
	}
	p = append(p, colorParams(fg, true)...)
	p = append(p, colorParams(bg, false)...)
	return "\x1b[" + strings.Join(p, ";") + "m"
}

// colorParams renders one color as SGR params, or nil for the default (already
// covered by the leading reset). base is 30 for foreground, 40 for background.
func colorParams(c vt10x.Color, fg bool) []string {
	base := 40
	if fg {
		base = 30
	}
	switch {
	case c == vt10x.DefaultFG || c == vt10x.DefaultBG:
		return nil
	case c < 8:
		return []string{strconv.Itoa(base + int(c))}
	case c < 16:
		return []string{strconv.Itoa(base + 60 + int(c-8))} // bright: 90/100 range
	case c < 256:
		return []string{strconv.Itoa(base + 8), "5", strconv.Itoa(int(c))} // 256-color
	default: // truecolor packed as r<<16 | g<<8 | b
		r, g, b := (c>>16)&0xff, (c>>8)&0xff, c&0xff
		return []string{strconv.Itoa(base + 8), "2", strconv.Itoa(int(r)), strconv.Itoa(int(g)), strconv.Itoa(int(b))}
	}
}
