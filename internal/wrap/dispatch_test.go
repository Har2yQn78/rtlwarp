package wrap

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Har2yQn78/rtlwrap/internal/shape"
)

// dispatch routes normal output to the pipe and alt-screen output to the
// termstate engine, passing the alt toggles through to switch real buffers.
func TestDispatchRoutesAltScreen(t *testing.T) {
	var buf bytes.Buffer
	d := newDispatcher(&buf, 20, 5)
	_, _ = d.Write([]byte("normal رفت\n"))      // pipe path (scrolling)
	_, _ = d.Write([]byte("\x1b[?1049h"))       // enter alt → passthrough
	_, _ = d.Write([]byte("\x1b[H\x1b[2Jسلام")) // engine path (grid redraw)
	_, _ = d.Write([]byte("\x1b[?1049l"))       // exit alt → passthrough
	_ = d.Close()

	out := buf.String()
	if !strings.Contains(out, "\x1b[?1049h") || !strings.Contains(out, "\x1b[?1049l") {
		t.Fatalf("alt toggles not passed through: %q", out)
	}
	if !strings.Contains(out, shape.Shape("رفت")) {
		t.Errorf("pipe did not shape scrolling RTL: %q", out)
	}
	if !strings.Contains(out, shape.Shape("سلام")) {
		t.Errorf("engine did not reshape alt-screen RTL: %q", out)
	}
}

// A toggle split across two writes must still be recognized (tokenizer holds
// the incomplete escape).
func TestDispatchToggleSplitAcrossWrites(t *testing.T) {
	var buf bytes.Buffer
	d := newDispatcher(&buf, 20, 5)
	_, _ = d.Write([]byte("\x1b[?10"))
	_, _ = d.Write([]byte("49h\x1b[H\x1b[2Jسلام"))
	_ = d.Close()
	if !strings.Contains(buf.String(), shape.Shape("سلام")) {
		t.Errorf("split toggle broke alt detection: %q", buf.String())
	}
}

func TestAltToggle(t *testing.T) {
	cases := []struct {
		in        string
		enter, ok bool
	}{
		{"\x1b[?1049h", true, true},
		{"\x1b[?1049l", false, true},
		{"\x1b[?47h", true, true},
		{"\x1b[?25h", false, false}, // cursor show, not alt-screen
		{"\x1b[0m", false, false},   // SGR
	}
	for _, c := range cases {
		enter, ok := altToggle([]byte(c.in))
		if enter != c.enter || ok != c.ok {
			t.Errorf("altToggle(%q) = (%v,%v), want (%v,%v)", c.in, enter, ok, c.enter, c.ok)
		}
	}
}
