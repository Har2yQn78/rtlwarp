# Phase 3 — terminal-state library decision + approach

**Status: implemented.** `internal/termstate` (the `Engine`) does the grid
reshaping; `internal/wrap/dispatch.go` routes alt-screen spans to it and normal
output to the Phase 2 pipe. The approach below is what shipped.

## Decision: use `github.com/hinshun/vt10x`

Bake-off between the two candidates from the design doc §4:

| | vt10x | bubbleterm |
|---|---|---|
| Shape | standalone VT emulator: `io.Writer` in, `Cell(x,y) Glyph` grid out | a **Bubble Tea `Model`** — a terminal widget to embed *inside* a TUI app |
| Deps | near-zero (`x/sys`, `x/sync`) | whole charm stack (bubbletea v2, ultraviolet, x/ansi, x/vt, colorprofile, …) |
| Fit for rtlwrap | exact — we feed raw child output, read the grid, reshape, render | wrong — we are a transparent passthrough, not a Bubbletea app |
| Maintenance | last commit 2022 | active |

**Chosen: vt10x.** rtlwrap is a passthrough wrapper, not a TUI app, so bubbleterm's
Bubble Tea integration is the wrong abstraction and its dependency weight is
unjustified. (The real standalone emulator under bubbleterm is
`charmbracelet/x/vt` — the maintained alternative if vt10x ever blocks us, but it
still pulls charm deps.) vt10x's staleness is low-risk: it emulates a *stable*
spec (VT100/xterm), and Phase 3 only needs grid read-back, not new features.

Smoke-tested: feeding `"\x1b[31mhi\x1b[0m سلام"` yields a grid holding the runes in
**logical order** with per-cell FG/BG and a tracked cursor — precisely what
reshaping needs.

## Approach (when Phase 3 is built)

vt10x stores the screen as a cell grid in **logical** order. The child's raw
output feeds vt10x (`Terminal.Write`); rtlwrap then renders the grid to the real
terminal, reshaping per row:

1. Feed child output → vt10x maintains the authoritative cell grid + cursor.
2. On change, for each dirty row: read its runes (logical) + per-cell colors.
3. Run the RTL run(s) in that row through `shape.Shape`, remapping colors to the
   reordered positions.
4. Emit the row with correct cursor positioning; leave LTR-only rows untouched.

This fixes the redraw/interactive case (Claude Code) that Phase 2 punts on —
because reshaping happens against real screen state, not a raw token stream.

**Out of scope even for Phase 3:** the monospace cell gap between joined letters
(font/terminal rendering, see `limitations.md`).
