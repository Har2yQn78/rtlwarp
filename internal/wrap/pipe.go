package wrap

import (
	"io"

	"rtlwrap/internal/parser"
	"rtlwrap/internal/shape"
)

// pipe is an io.Writer that sits on the child's output: it tokenizes, shapes
// only TEXT runs, and passes ANSI/CONTROL bytes through untouched.
//
// Shaping is line-based, so consecutive TEXT is accumulated and shaped as a
// unit at the next boundary (any ANSI or CONTROL token — a newline is a
// CONTROL byte, so lines flush naturally).
//
// ponytail: text with no RTL is flushed at end of every Write (identity shape,
// zero added latency for English). Text containing RTL is held across Writes
// until a boundary, so a word split across two PTY reads still joins/reorders
// as one line. Trade-off: an RTL partial line with no trailing newline waits
// for the next read or Close — acceptable for Phase 2 (scrolling text), fixed
// properly by Phase 3 termstate.
type pipe struct {
	tk   parser.Tokenizer
	text []byte // accumulated TEXT awaiting a boundary
	w    io.Writer
}

func newPipe(w io.Writer) *pipe { return &pipe{w: w} }

func (p *pipe) Write(chunk []byte) (int, error) {
	for _, t := range p.tk.Push(chunk) {
		if t.Kind == parser.TEXT {
			p.text = append(p.text, t.Bytes...)
			continue
		}
		if err := p.flushText(); err != nil {
			return 0, err
		}
		if _, err := p.w.Write(t.Bytes); err != nil {
			return 0, err
		}
	}
	if !hasRTL(p.text) { // safe to emit LTR incrementally
		if err := p.flushText(); err != nil {
			return 0, err
		}
	}
	return len(chunk), nil
}

// Close flushes the tokenizer's held bytes and any remaining text. Call once
// at EOF.
func (p *pipe) Close() error {
	for _, t := range p.tk.Flush() {
		if t.Kind == parser.TEXT {
			p.text = append(p.text, t.Bytes...)
			continue
		}
		if err := p.flushText(); err != nil {
			return err
		}
		if _, err := p.w.Write(t.Bytes); err != nil {
			return err
		}
	}
	return p.flushText()
}

func (p *pipe) flushText() error {
	if len(p.text) == 0 {
		return nil
	}
	_, err := io.WriteString(p.w, shape.Shape(string(p.text)))
	p.text = p.text[:0]
	return err
}

// hasRTL reports whether s contains any right-to-left character (Hebrew,
// Arabic/Persian, and related blocks, plus Arabic presentation forms).
func hasRTL(s []byte) bool {
	for _, r := range string(s) {
		switch {
		case r >= 0x0590 && r <= 0x08FF, // Hebrew … Arabic Extended
			r >= 0xFB1D && r <= 0xFDFF, // Hebrew + Arabic presentation forms A
			r >= 0xFE70 && r <= 0xFEFF: // Arabic presentation forms B
			return true
		}
	}
	return false
}
