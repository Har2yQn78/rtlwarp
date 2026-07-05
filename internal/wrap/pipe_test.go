package wrap

import (
	"bytes"
	"testing"

	"rtlwrap/internal/shape"
)

// feed writes in through a pipe in the given chunk sizes and returns output.
func feed(in string, chunk int) string {
	var buf bytes.Buffer
	p := newPipe(&buf)
	b := []byte(in)
	if chunk <= 0 {
		_, _ = p.Write(b)
	} else {
		for i := 0; i < len(b); i += chunk {
			end := i + chunk
			if end > len(b) {
				end = len(b)
			}
			_, _ = p.Write(b[i:end])
		}
	}
	_ = p.Close()
	return buf.String()
}

func TestPipePassesAnsiThrough(t *testing.T) {
	in := "\x1b[31mhello\x1b[0m\n"
	if got := feed(in, 0); got != in { // pure LTR: bytes unchanged
		t.Errorf("ANSI/LTR mangled:\n got %q\nwant %q", got, in)
	}
}

func TestPipeShapesText(t *testing.T) {
	in := "سلام\n"
	want := shape.Shape("سلام") + "\n" // newline is CONTROL, passed through
	if got := feed(in, 0); got != want {
		t.Errorf("shape mismatch:\n got %q\nwant %q", got, want)
	}
}

// RTL text split across writes must still shape as one line (not per-fragment).
func TestPipeRTLAcrossChunks(t *testing.T) {
	in := "سلام دنیا\n"
	want := feed(in, 0)
	for _, ch := range []int{1, 2, 3} {
		if got := feed(in, ch); got != want {
			t.Errorf("chunk=%d: RTL split shaped differently:\n got %q\nwant %q", ch, got, want)
		}
	}
}

// Color reset around a whole RTL line is the common case and must work.
func TestPipeColoredRTLLine(t *testing.T) {
	in := "\x1b[1mسلام\x1b[0m\n"
	want := "\x1b[1m" + shape.Shape("سلام") + "\x1b[0m\n"
	if got := feed(in, 0); got != want {
		t.Errorf("colored RTL line:\n got %q\nwant %q", got, want)
	}
}
