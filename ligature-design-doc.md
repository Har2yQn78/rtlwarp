# Project Design Doc: ligature — RTL Terminal Rendering Fix

## 1. Purpose

Terminals draw characters in the order and shape they receive. Most don't reorder or join Persian/Arabic script themselves. Programs send correctly-formed RTL text (joined letters, right-to-left reading order), and terminals without bidi support just stamp each character left-to-right on the grid — producing reversed, disconnected text.

This is what happens with Claude Code's Persian output today, and the same underlying gap affects most terminal programs that weren't built with RTL scripts in mind.

**This project fixes that without requiring any cooperation from the program being run.** It works by sitting between the program and the real terminal, correcting text in transit. One deliverable: a standalone wrapper binary you run in front of any existing terminal program.

**Positioning (see Section 8 before writing any README copy):** this is a PTY wrapper that enables correct RTL shaping and bidirectional rendering for terminal applications that don't support it — not a blanket claim that it "fixes RTL rendering" everywhere, in every case.

## 2. Name

**ligature** — the correct typography term for joined letterforms, which is exactly what Arabic/Persian shaping produces.

- Internal shaping code: `shape.Shape(...)`
- CLI usage: `ligature claude`, `ligature codex`, `ligature git log`

## 3. Architecture overview

The key design decision: **never reason about "RTL runs" directly against a raw byte stream.** Instead, tokenize first, then only transform one category of token.

```
┌─────────────┐    keystrokes     ┌──────────────┐   keystrokes    ┌─────────────┐
│  Real user   │ ───────────────► │              │ ──────────────► │  Child proc  │
│  keyboard    │                  │   ligature   │                 │ (claude,     │
│              │ ◄─────────────── │  (this tool) │ ◄────────────── │  codex, git, │
│ Real terminal│   fixed output   │              │   raw output    │  etc.)       │
└─────────────┘                  └──────────────┘                 └─────────────┘
                                         │
                                         ▼
                        raw byte stream from child process
                                         │
                                         ▼
                        ┌───────────────────────────────┐
                        │  internal/parser (Phase 2)     │
                        │  tokenizes into a sequence of: │
                        │   ANSI | TEXT | CONTROL        │
                        └───────────────────────────────┘
                                         │
                    only TEXT tokens are ever touched
                                         ▼
                        ┌───────────────────────────────┐
                        │  internal/shape (Phase 1)      │
                        │  bidi reorder + letter shaping │
                        │  applied ONLY to TEXT tokens   │
                        └───────────────────────────────┘
                                         │
                    ANSI/CONTROL tokens rejoin untouched
                                         ▼
                              written to real terminal
```

Why this matters: ANSI escape sequences and control bytes are structurally identified by the tokenizer *before* any shaping logic runs, so shaping code never sees them and can never accidentally corrupt a cursor-movement or color code. This replaces the earlier framing of "detect RTL runs in the byte stream," which required guessing where escape sequences ended — a much riskier approach.

Three internal layers, built in this order:

- **`internal/parser`** — classifies every byte from the child process into `ANSI`, `TEXT`, or `CONTROL`. Knows nothing about Persian/Arabic or shaping.
- **`internal/shape`** — pure text transformation, operates only on `TEXT` tokens handed to it by the parser. No knowledge of terminals, PTYs, or ANSI codes.
- **`internal/termstate`** (Phase 3) — tracks actual screen/cursor state so `TEXT` tokens split across redraws can be reshaped correctly.

## 4. Packages needed

| Package | Purpose | Phase |
|---|---|---|
| `github.com/benoitkugler/textlayout/fribidi` | Core bidi reordering + Arabic/Persian letter shaping engine (Go port of fribidi) | 1 |
| `golang.org/x/text/unicode/bidi` | Bidi class lookups, cross-checking | 1 |
| `github.com/rivo/uniseg` | Grapheme cluster segmentation — needed for correct cursor/width math, and for not mangling emoji/ZWJ sequences/combining marks (see Section 8) | 1, 3 |
| `github.com/mattn/go-runewidth` | Terminal cell-width calculation | 1, 3 |
| `github.com/creack/pty` | Spawn child process attached to a pseudo-terminal | 2 |
| `golang.org/x/term` | Raw terminal mode, get terminal size | 2 |
| terminal-state library — **not locked in yet** | Tracks cell grid, cursor, colors from a raw ANSI stream. Candidates: `github.com/hinshun/vt10x` (works, but not aggressively maintained), `github.com/taigrr/bubbleterm` (newer, built explicitly to pair with `creack/pty` and the Bubbletea ecosystem). **Action item: bake-off both before Phase 3 starts, don't commit now.** | 3 |

## 5. Phases (complete plan)

### Phase 0 — Validate the core transform
Purpose: prove `fribidi`'s Go port actually produces correct output before building anything around it.
- Feed it known test strings: pure Persian, pure Arabic, mixed Persian+English, Persian+numbers, Persian+punctuation.
- Also test known-hard Unicode cases early: emoji, ZWJ sequences, combining marks — don't discover these are unhandled in Phase 2.
- Deliverable: `shape_test.go` with real strings. No public API yet.

### Phase 1 — `internal/shape` engine
Purpose: a clean, minimal internal package everything else depends on.
- API: `shape.Shape(s string) string`
- Handles mixed-direction lines correctly.
- Correct width reporting for shaped/ligated output — needed later for cursor math.
- Fully testable in isolation, no terminal/PTY involved yet.

### Phase 2 — Tokenizing PTY wrapper, static/scrolling text only
Purpose: fix RTL rendering for Claude Code, Codex, and any other CLI program — the actual product, v0.1.
- Spawn the target program attached to a PTY (`creack/pty`).
- `internal/parser` tokenizes the raw output stream into `ANSI | TEXT | CONTROL`, in that explicit sequence — never treat the stream as "mostly text with some escape codes in it."
- Only `TEXT` tokens are passed to `internal/shape`. `ANSI` and `CONTROL` tokens are written back out byte-for-byte, never inspected for content.
- Forward `SIGWINCH` (terminal resize) from the real terminal to the child's PTY.
- Scope explicitly limited to content that scrolls (won't yet handle in-place redraws/spinners correctly — documented as a known limitation, not silently broken).
- Deliverable: `ligature claude`, `ligature codex`, `ligature git log`, working correctly for the common case.

### Phase 3 — Redraw-safe wrapper (handles live-updating UI)
Purpose: fix the flicker/misalignment risk from streaming responses and in-place redraws (spinners, status lines).
- Finalize the terminal-state library decision (see Section 4) before starting this phase.
- Replace the naive token-stream approach with a proper terminal-state tracker that knows what's actually on each row/column at any moment.
- Reshape `TEXT` tokens against the tracked screen state, so partial words across redraw boundaries are handled correctly.
- This is the most technically demanding phase — budget the most time here.

### Phase 4 — Packaging and distribution
Purpose: make it trivially installable.
- Single static binary, cross-compiled (Linux/macOS/Windows-via-WSL).
- Homebrew tap, AUR package (Fedora first), `go install` support.
- GoReleaser-based GitHub Actions release pipeline.
- README copy follows Section 8's framing exactly — no "fixes RTL rendering" blanket claim.
- `docs/terminal-compatibility.md` — document which terminals already attempt partial bidi themselves, so users don't double-process.
- `docs/limitations.md` — emoji/ZWJ/grapheme-cluster edge cases, redraw-heavy apps before Phase 3 lands, terminals with native partial bidi.

## 6. Folder structure

```
ligature/
├── go.mod
├── go.sum
├── README.md
├── LICENSE
├── CHANGELOG.md
├── Makefile
├── internal/                     # not intended for external import — the CLI is the product
│   ├── parser/                   # Phase 2 — tokenizes raw stream into ANSI | TEXT | CONTROL
│   │   ├── tokenizer.go
│   │   └── tokenizer_test.go
│   ├── shape/                    # Phase 0/1 — core shaping engine
│   │   ├── shape.go              # Shape(string) string
│   │   ├── bidi.go               # wraps fribidi bidi reordering
│   │   ├── arabic.go             # Arabic/Persian presentation-form shaping
│   │   ├── width.go              # cell-width calc for shaped/ligated text
│   │   ├── shape_test.go
│   │   └── testdata/             # known-good/broken reference strings, incl. emoji/ZWJ cases
│   ├── termstate/                # Phase 3 — terminal state tracking
│   │   ├── screen.go             # cell grid, cursor tracking
│   │   ├── reshape.go            # applies shape.Shape() per tracked row safely
│   │   └── termstate_test.go
│   └── wrap/                     # Phase 2 & 3 — the PTY wrapper engine, wires the above together
│       ├── pty.go                # spawns child process on a PTY
│       ├── pipe.go               # Phase 2: parser + shape, token-stream based
│       ├── engine.go             # Phase 3: routes through termstate instead
│       ├── resize.go             # SIGWINCH forwarding
│       └── wrap_test.go
├── cmd/
│   └── ligature/
│       └── main.go               # CLI entrypoint: `ligature -- claude`
├── docs/
│   ├── architecture.md
│   ├── terminal-compatibility.md
│   └── limitations.md
└── .github/
    └── workflows/
        ├── test.yml
        └── release.yml           # goreleaser: brew, AUR, cross-platform binaries
```

## 7. Suggested build order (practical)

1. Phase 0 + 1 first, entirely — get `shape.Shape()` provably correct, including emoji/ZWJ/combining-mark test cases, before touching PTYs at all.
2. Phase 2 — build `internal/parser` as a genuinely separate, independently-tested component from `internal/shape`. This is the part that fixes your actual daily Claude Code/Codex annoyance. Ship as usable v0.1, redraw limitation documented.
3. Before Phase 3: spend a short, explicit bake-off on the terminal-state library choice. Don't inherit whatever was easiest to wire up in Phase 2.
4. Phase 3 — the deep technical phase.
5. Phase 4 — start the GitHub Actions release pipeline as soon as Phase 2 works.

## 8. Known limitations and honest scope (read before writing README copy)

This list exists so the project's public claims stay defensible. Update it as new edge cases are found.

- **Terminals with partial native bidi support** (some iTerm2/Kitty configurations) may double-process text if `ligature` also reshapes it. Detection or explicit user configuration is needed — not solved by default in Phase 2.
- **Emoji, ZWJ sequences, and combining marks** are genuinely hard Unicode cases independent of bidi. `uniseg`-based grapheme clustering helps, but this needs dedicated test coverage, not an assumption that "shaping mixed text" automatically handles it.
- **Redraw-heavy applications** (full-screen TUIs, spinners, live status lines) are not reliably correct until Phase 3 ships. Phase 2 should say so explicitly rather than imply general correctness.
- **Marketing language:** use *"a PTY wrapper that enables correct RTL shaping and bidirectional rendering for terminal applications that don't support it"* — not *"fixes RTL rendering."* The former is accurate and still a strong, specific claim. The latter invites a stream of edge-case bug reports framed as broken promises rather than known limitations.