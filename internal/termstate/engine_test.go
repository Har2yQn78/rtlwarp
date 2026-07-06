package termstate

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Har2yQn78/rtlwrap/internal/shape"
)

// A cursor-positioned redraw (home + clear + Persian) must land the reshaped
// presentation forms in the output — the case the Phase 2 pipe can't handle.
func TestEngineReshapesRedraw(t *testing.T) {
	var buf bytes.Buffer
	e := New(&buf, 20, 3)
	if _, err := e.Write([]byte("\x1b[H\x1b[2Jسلام")); err != nil {
		t.Fatal(err)
	}
	want := shape.Shape("سلام") // "ﻡﻼﺳ"
	if !strings.Contains(buf.String(), want) {
		t.Errorf("reshaped RTL missing\n got %q\nwant substr %q", buf.String(), want)
	}
}

// A whole-run color must survive reshaping: the color moves onto the reordered
// runes via the visualToLogical map, staying attached to the same letters.
func TestEngineColorRemap(t *testing.T) {
	var buf bytes.Buffer
	e := New(&buf, 20, 3)
	if _, err := e.Write([]byte("\x1b[H\x1b[2J\x1b[31mسلام\x1b[0m")); err != nil {
		t.Fatal(err)
	}
	want := "\x1b[0;31m" + shape.Shape("سلام") // red SGR then the reshaped run
	if !strings.Contains(buf.String(), want) {
		t.Errorf("color not remapped onto reshaped run\n got %q\nwant substr %q", buf.String(), want)
	}
}

// Pure-LTR content passes through unreordered.
func TestEngineLTRPassthrough(t *testing.T) {
	var buf bytes.Buffer
	e := New(&buf, 20, 3)
	if _, err := e.Write([]byte("\x1b[H\x1b[2Jhello")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("LTR text mangled: %q", buf.String())
	}
}

// An unchanged frame re-renders no row bodies (only cursor repositioning).
func TestEngineDiffSkipsUnchanged(t *testing.T) {
	var buf bytes.Buffer
	e := New(&buf, 20, 3)
	_, _ = e.Write([]byte("\x1b[H\x1b[2Jhi"))
	buf.Reset()
	_, _ = e.Write([]byte("")) // no state change
	if strings.Contains(buf.String(), "hi") {
		t.Errorf("unchanged row was re-emitted: %q", buf.String())
	}
}

func TestVisualX(t *testing.T) {
	// visual rune 0 came from logical 3, rune 1 from logical 2, ...
	v2l := []int{3, 2, 1, 0}
	for logical, wantVis := range map[int]int{0: 3, 3: 0, 2: 1} {
		if got := visualX(v2l, logical, 20); got != wantVis {
			t.Errorf("visualX(logical=%d) = %d, want %d", logical, got, wantVis)
		}
	}
	if got := visualX(v2l, 99, 20); got != 19 { // past end clamps to cols-1
		t.Errorf("visualX past-end = %d, want 19", got)
	}
}
