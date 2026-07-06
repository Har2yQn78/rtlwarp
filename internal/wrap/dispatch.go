package wrap

import (
	"io"
	"sync"

	"github.com/Har2yQn78/rtlwrap/internal/parser"
	"github.com/Har2yQn78/rtlwrap/internal/termstate"
)

type dispatcher struct {
	mu     sync.Mutex // guards alt/engine/size against the SIGWINCH resize
	tk     parser.Tokenizer
	out    io.Writer
	pipe   *pipe
	engine *termstate.Engine
	alt    bool
	cols   int
	rows   int
	buf    []byte // bytes accrued for the current consumer within one Write
}

func newDispatcher(out io.Writer, cols, rows int) *dispatcher {
	return &dispatcher{out: out, pipe: newPipe(out), cols: cols, rows: rows}
}

func (d *dispatcher) Write(chunk []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, t := range d.tk.Push(chunk) {
		if t.Kind == parser.ANSI {
			if enter, ok := altToggle(t.Bytes); ok {
				if err := d.flush(); err != nil { // hand off buffered span
					return 0, err
				}
				if _, err := d.out.Write(t.Bytes); err != nil { // switch real buffers
					return 0, err
				}
				d.setAlt(enter)
				continue
			}
		}
		d.buf = append(d.buf, t.Bytes...)
	}
	return len(chunk), d.flush()
}

// Close flushes held bytes at EOF. Call once when the child closes the PTY.
func (d *dispatcher) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, t := range d.tk.Flush() {
		d.buf = append(d.buf, t.Bytes...)
	}
	if err := d.flush(); err != nil {
		return err
	}
	return d.pipe.Close()
}

// Resize updates the size used for new engines and resizes the active one.
func (d *dispatcher) Resize(cols, rows int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cols, d.rows = cols, rows
	if d.engine != nil {
		d.engine.Resize(cols, rows)
	}
}

// flush sends the accrued span to whichever renderer is currently active.
func (d *dispatcher) flush() error {
	if len(d.buf) == 0 {
		return nil
	}
	var err error
	if d.alt {
		_, err = d.engine.Write(d.buf)
	} else {
		_, err = d.pipe.Write(d.buf)
	}
	d.buf = d.buf[:0]
	return err
}

func (d *dispatcher) setAlt(enter bool) {
	if enter == d.alt {
		return
	}
	d.alt = enter
	if enter {
		// Fresh engine per alt session: the real terminal just cleared to its
		// alt buffer, so the grid starts blank and matches.
		d.engine = termstate.New(d.out, d.cols, d.rows)
	} else {
		d.engine = nil
	}
}

// altToggle reports whether b is an alt-screen enter/exit private-mode set
// (ESC[?<n>h) or reset (ESC[?<n>l) for n in {1049, 1047, 47}. enter is true for
// the set form.
func altToggle(b []byte) (enter, ok bool) {
	if len(b) < 5 || b[0] != 0x1b || b[1] != '[' || b[2] != '?' {
		return false, false
	}
	last := b[len(b)-1]
	if last != 'h' && last != 'l' {
		return false, false
	}
	switch string(b[3 : len(b)-1]) {
	case "1049", "1047", "47":
		return last == 'h', true
	}
	return false, false
}
