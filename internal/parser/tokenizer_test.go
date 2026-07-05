package parser

import (
	"bytes"
	"testing"
)

type exp struct {
	kind  Kind
	bytes string
}

var cases = []struct {
	name string
	in   string
	want []exp
}{
	{"plain text", "hello", []exp{{TEXT, "hello"}}},
	{"persian", "سلام", []exp{{TEXT, "سلام"}}},
	{"newline splits control", "a\nb", []exp{{TEXT, "a"}, {CONTROL, "\n"}, {TEXT, "b"}}},
	{"csi color", "\x1b[31mred\x1b[0m", []exp{
		{ANSI, "\x1b[31m"}, {TEXT, "red"}, {ANSI, "\x1b[0m"}}},
	{"cursor move", "\x1b[2Jx", []exp{{ANSI, "\x1b[2J"}, {TEXT, "x"}}},
	{"osc title bel", "\x1b]0;title\x07ok", []exp{
		{ANSI, "\x1b]0;title\x07"}, {TEXT, "ok"}}},
	{"osc title st", "\x1b]0;t\x1b\\ok", []exp{
		{ANSI, "\x1b]0;t\x1b\\"}, {TEXT, "ok"}}},
	{"two byte escape", "\x1b7hi", []exp{{ANSI, "\x1b7"}, {TEXT, "hi"}}},
	{"charset designator", "\x1b(Bhi", []exp{{ANSI, "\x1b(B"}, {TEXT, "hi"}}},
	{"cr lf", "\r\n", []exp{{CONTROL, "\r"}, {CONTROL, "\n"}}},
	{"persian with color", "\x1b[1mسلام\x1b[0m", []exp{
		{ANSI, "\x1b[1m"}, {TEXT, "سلام"}, {ANSI, "\x1b[0m"}}},
}

func collect(in string, chunk int) []Token {
	var tk Tokenizer
	var got []Token
	b := []byte(in)
	if chunk <= 0 { // one-shot
		got = append(got, tk.Push(b)...)
	} else {
		for i := 0; i < len(b); i += chunk {
			end := i + chunk
			if end > len(b) {
				end = len(b)
			}
			got = append(got, tk.Push(b[i:end])...)
		}
	}
	return mergeText(append(got, tk.Flush()...))
}

// mergeText coalesces adjacent TEXT tokens. The tokenizer may emit a text run
// in per-chunk fragments (it only guarantees runes/sequences aren't split);
// the pipe reassembles consecutive TEXT before shaping, so that's the level
// the invariant holds at.
func mergeText(in []Token) []Token {
	var out []Token
	for _, tk := range in {
		if tk.Kind == TEXT && len(out) > 0 && out[len(out)-1].Kind == TEXT {
			out[len(out)-1].Bytes = append(out[len(out)-1].Bytes, tk.Bytes...)
			continue
		}
		out = append(out, tk)
	}
	return out
}

func check(t *testing.T, name string, got []Token, want []exp) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %d tokens, want %d: %+v", name, len(got), len(want), got)
		return
	}
	for i, w := range want {
		if got[i].Kind != w.kind || !bytes.Equal(got[i].Bytes, []byte(w.bytes)) {
			t.Errorf("%s: token %d = {%s %q}, want {%s %q}",
				name, i, got[i].Kind, got[i].Bytes, w.kind, w.bytes)
		}
	}
}

func TestTokenizeOneShot(t *testing.T) {
	for _, c := range cases {
		check(t, c.name, collect(c.in, 0), c.want)
	}
}

// The firewall must survive PTY chunking: feeding one byte at a time must
// yield exactly the same tokens as one-shot. This is where a naive splitter
// corrupts escape sequences.
func TestTokenizeByteByByte(t *testing.T) {
	for _, c := range cases {
		check(t, c.name+" (1-byte chunks)", collect(c.in, 1), c.want)
		check(t, c.name+" (2-byte chunks)", collect(c.in, 2), c.want)
	}
}

// Flush must emit incomplete trailing bytes rather than drop them.
func TestFlushIncomplete(t *testing.T) {
	var tk Tokenizer
	got := tk.Push([]byte("hi\x1b[")) // CSI with no final byte yet
	got = append(got, tk.Flush()...)
	check(t, "incomplete csi flushed", got, []exp{{TEXT, "hi"}, {ANSI, "\x1b["}})
}
