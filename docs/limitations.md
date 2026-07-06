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

## Redraw-heavy / interactive apps

rtlwrap runs two renderers and switches between them automatically:

- **Scrolling / static output** (`rtlwrap cat file.fa`, `rtlwrap git log`,
  program logs) — shaped line-by-line as it scrolls, streamed into the real
  terminal's scrollback. Works well.
- **Full-screen TUIs that use the alternate screen** (vim, less, and
  full-screen interactive apps) — reshaped against the live terminal-state grid
  (Phase 3 `termstate`): rtlwrap feeds the child's output into a virtual
  terminal, then re-emits each row with its RTL runs reshaped and colors
  remapped onto the reordered cells. This handles cursor-positioned repaints
  that the scrolling renderer cannot.

**Remaining gaps:**

- **Apps that repaint the *normal* screen in place** (no alternate-screen
  buffer — some spinners, progress bars, status lines) still go through the
  scrolling renderer, so in-place cursor moves inside a line can shape
  fragments independently. Only alternate-screen apps get the grid renderer.
  Whether a given interactive app (e.g. Claude Code) is fully correct depends on
  whether it uses the alternate screen.
- **Cursor position in a reshaped RTL row** can sit one cell off per lam-alef
  ligature to its left (the zero-width filler is stripped from display but not
  yet subtracted from the cursor column).
- **Symptom in the scrolling renderer:** a partial RTL line with no trailing
  newline is held until the next read or EOF (so a word split across two PTY
  reads still joins as one line).

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
