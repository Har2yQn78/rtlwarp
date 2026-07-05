package parser

import "unicode/utf8"

type Kind uint8

const (
	TEXT    Kind = iota // printable UTF-8 run (the only kind shaping touches)
	ANSI                // an escape sequence, emitted byte-for-byte
	CONTROL             // a single C0 control byte or DEL, byte-for-byte
)

func (k Kind) String() string {
	switch k {
	case TEXT:
		return "TEXT"
	case ANSI:
		return "ANSI"
	default:
		return "CONTROL"
	}
}

type Token struct {
	Kind  Kind
	Bytes []byte
}

// Tokenizer is stateful so it can span chunk boundaries: a sequence or rune
// split across two Push calls is completed on the next one.
type Tokenizer struct {
	buf []byte // unconsumed tail carried between Push calls
}

// Push feeds a chunk and returns every complete token in it. Any incomplete
// trailing sequence (partial ANSI escape or partial UTF-8 rune) is retained
// and completed by the next Push, or emitted as-is by Flush.
func (t *Tokenizer) Push(chunk []byte) []Token {
	t.buf = append(t.buf, chunk...)
	var toks []Token
	i := 0
	for i < len(t.buf) {
		b := t.buf[i]
		switch {
		case b == 0x1B: // ESC → ANSI escape sequence
			n, ok := ansiLen(t.buf[i:])
			if !ok {
				i = holdFrom(t, i) // incomplete, wait for more bytes
				return toks
			}
			toks = append(toks, tok(ANSI, t.buf[i:i+n]))
			i += n
		case b < 0x20 || b == 0x7F: // other C0 control or DEL
			toks = append(toks, tok(CONTROL, t.buf[i:i+1]))
			i++
		default: // printable → maximal TEXT run
			n, incomplete := textLen(t.buf[i:])
			if n > 0 {
				toks = append(toks, tok(TEXT, t.buf[i:i+n]))
				i += n
			}
			if incomplete { // partial rune at end, hold it
				i = holdFrom(t, i)
				return toks
			}
		}
	}
	t.buf = t.buf[:0]
	return toks
}

// Flush emits any buffered incomplete bytes byte-for-byte at EOF, so nothing
// is ever dropped. Call once after the stream closes.
func (t *Tokenizer) Flush() []Token {
	if len(t.buf) == 0 {
		return nil
	}
	k := TEXT
	switch b := t.buf[0]; {
	case b == 0x1B:
		k = ANSI
	case b < 0x20 || b == 0x7F:
		k = CONTROL
	}
	out := []Token{tok(k, t.buf)}
	t.buf = t.buf[:0]
	return out
}

// holdFrom retains buf[i:] for the next Push and returns the new length.
func holdFrom(t *Tokenizer, i int) int {
	t.buf = append(t.buf[:0], t.buf[i:]...)
	return len(t.buf)
}

func tok(k Kind, b []byte) Token {
	return Token{Kind: k, Bytes: append([]byte(nil), b...)}
}

// textLen returns the count of leading bytes in b that form complete
// printable text, stopping before any control/ESC byte. incomplete is true
// when b ends mid-rune (a multibyte sequence that may finish in a later chunk).
func textLen(b []byte) (n int, incomplete bool) {
	j := 0
	for j < len(b) {
		c := b[j]
		if c == 0x1B || c < 0x20 || c == 0x7F {
			return j, false // control/ESC ends the text run
		}
		if c < 0x80 {
			j++ // ASCII printable
			continue
		}
		if !utf8.FullRune(b[j:]) {
			return j, true // truncated multibyte rune at end of buffer
		}
		_, size := utf8.DecodeRune(b[j:]) // full rune (keep even if malformed)
		j += size
	}
	return j, false
}

// ansiLen reports the length of the escape sequence starting at b[0]==ESC and
// whether it is complete. Covers CSI, OSC/DCS/SOS/PM/APC string sequences,
// charset designators, and two-byte escapes — the sequences real terminals
// actually emit.
func ansiLen(b []byte) (n int, complete bool) {
	if len(b) < 2 {
		return 0, false
	}
	switch b[1] {
	case '[': // CSI: params/intermediates then a final byte 0x40-0x7E
		for j := 2; j < len(b); j++ {
			if b[j] >= 0x40 && b[j] <= 0x7E {
				return j + 1, true
			}
		}
		return 0, false
	case ']', 'P', 'X', '^', '_': // string sequence, ends at BEL or ST (ESC \)
		for j := 2; j < len(b); j++ {
			if b[j] == 0x07 {
				return j + 1, true
			}
			if b[j] == 0x1B {
				if j+1 < len(b) && b[j+1] == '\\' {
					return j + 2, true
				}
				return 0, false // ESC with no terminator yet
			}
		}
		return 0, false
	case '(', ')', '*', '+', '%': // charset designator: ESC, intermediate, 1 byte
		if len(b) < 3 {
			return 0, false
		}
		return 3, true
	default: // two-byte escape (ESC 7, ESC =, ESC M, ...)
		return 2, true
	}
}
