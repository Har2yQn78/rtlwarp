# Limitations & honest scope

rtlwrap is a PTY wrapper that **enables correct RTL shaping and bidirectional
rendering for terminal applications that don't support it.** It is not a claim
that RTL renders perfectly everywhere. This document lists what it does not do,
so the project's promises stay defensible.

## Rendering — not fixable by rtlwrap

- **Letters have a small gap between them (not seamlessly cursive).**
  Arabic/Persian is a cursive, *proportional* script; a terminal is a
  *fixed-cell monospace grid*. rtlwrap emits the correct joined presentation
  forms (initial/medial/final/isolated), but each glyph still gets a whole
  cell of advance, so joined letters sit adjacent rather than flowing into one
  another. This is a font + terminal rendering limit, not something a
  byte-stream transformer can close. Phase 3 does **not** fix it either — it's
  a different layer.
  - *Improves with:* a terminal with good Arabic cell handling (Kitty, foot)
    and a font designed for it (Vazirmatn, Noto Naskh Arabic).
- **Some breaks between letters are correct.** ا د ذ ر ز و and friends never
  join to their left. A gap after them is proper Persian, not a bug.

## Redraw-heavy / interactive apps — needs Phase 3

- **Full-screen TUIs, spinners, live status lines, streaming input echo**
  (e.g. Claude Code's interactive prompt) are **not** reliably correct yet.
  rtlwrap currently shapes text line-by-line as it scrolls; an app that repaints
  in place with cursor-positioning escape sequences interleaves ANSI codes
  inside a line, and the fragments between them get shaped independently. Fixed
  properly by Phase 3 (terminal-state tracking), which reshapes against what is
  actually on each screen cell.
- **Symptom before Phase 3:** RTL input may not appear until a boundary
  character arrives (RTL text is held so a word split across two PTY reads
  still joins as one line).

**Works well today:** scrolling / static output — `rtlwrap cat file.fa`,
`rtlwrap git log`, program logs, plain command output.

## Mid-line color inside an RTL run

- A color change **inside** a single Persian word splits shaping at the escape
  sequence, so that word may shape per-segment. Whole-line color (set at start,
  reset at end) is correct. Also a Phase 3 concern.

## Unicode edge cases

- **Emoji, ZWJ sequences, combining marks** are preserved (not dropped or
  reordered into the RTL run), but exhaustive correctness across every emoji
  ZWJ family and combining-mark stack is not guaranteed. Covered by tests for
  the common cases only.

## Terminals with partial native bidi

- Some iTerm2 / Kitty configurations attempt their own bidi reordering. Running
  rtlwrap on top may double-process text (reorder an already-reordered stream).
  There is no auto-detection yet — disable one side manually.
