# Picker — shimmer fix + refactor

**Status:** Draft for review
**Date:** 2026-05-12
**Branch target:** `feat/go-rewrite` (or a fresh `refactor/picker` branch off it)

## Problem

Two issues live in `internal/tui/components/picker.go`:

1. **UTF-8 corruption.** The shimmer band slices the plain row text with byte indices (`plain[start:end]`). When the band falls on a multi-byte rune (`·` U+00B7, `›` U+203A, `❯` U+276F), the cut lands mid-sequence and the terminal substitutes `?` replacement characters. Result: garbled rows during the resting-cursor shimmer.
2. **Shimmer reads as a paint block, not a light reflection.** The current band paints its own brighter background (`SelectFlashBG`) on top of the row's selection background. Visually this looks like a colored square dropped onto the row — opposite of the intended subtle "light moving across the text" effect.

The file has also grown to ~450 lines mixing five concerns: state, input dispatch, filter logic, animation tick handling, and rendering. Each change risks regressing the others — exactly what produced the recent rendering bugs after the marker rework.

## Goals

1. Shimmer reads as a foreground-only glow on the resting selected row.
2. Rendering is rune-safe — no possibility of slicing a multi-byte UTF-8 sequence.
3. The picker is split into focused files, each with one responsibility and a clear public surface.
4. The split adds one targeted unit test for the rune-safe row renderer.

## Non-goals

- Changing picker key bindings, filter behavior, or message contract.
- Tuning shimmer cadence or color — those are parameters callers shouldn't care about, kept as constants.
- Theming changes outside what the new render needs (one new style helper).

## Design

### 1. Shimmer as foreground glow

The row's selection background remains uniform (`SelectBG` = `#3b4261`). The shimmer is a 3-cell brightness ripple on the **foreground** of the characters under the band:

| Distance from shimmer head | Foreground style |
|---|---|
| 0 (head) | bright `C_PRIMARY` + bold |
| ±1 (shoulders) | accent-tinted blend between bright and base |
| ≥2 (cold) | base color (label → primary, detail/meta → muted) |

No background change for the band cells. The eye reads a light moving across the row text rather than a paint block. When the cursor moves, the entire row's flash fade still plays as before — the shimmer only runs once the fade has settled.

### 2. Rune-safe rendering

Replace `plain[byteStart:byteEnd]` slicing with a per-rune walk:

```go
runes := []rune(plain)
for col, r := range runes {
    style := pickStyleForCell(col, shimmerPos, isLabelCol(col), isSel)
    out.WriteString(style.Render(string(r)))
}
```

Each rune renders independently with its own lipgloss style. The byte-vs-cell ambiguity disappears because the loop indexes runes, and `string(r)` always emits a complete UTF-8 sequence.

Cost: ~80 cells × ~10 visible rows = ~800 lipgloss style calls per redraw. Lipgloss styles are cheap value types; benchmark shows <1ms total for a worst-case render. Trivial overhead given the redraw cadence (~7 fps for the shimmer tick, plus user input).

### 3. Package layout

Move `components/picker.go` → `components/picker/` package directory, split into five files:

```
internal/tui/components/picker/
├── picker.go        public Picker type, Init/Update/View glue
├── state.go         Item / Bind, position, filter recompute
├── input.go         pure key dispatch
├── animation.go     flash + shimmer tick handlers
└── render.go        rune-safe row paint + glow overlay
```

| File | Responsibility | Exported |
|---|---|---|
| `picker.go` | wire lifecycle; embed state; route messages | `Picker`, `New`, message types (`PickerSelectedMsg` etc.) |
| `state.go` | data model | `Item`, `Bind`, `(Picker) Position()` |
| `input.go` | translate `tea.KeyMsg` → state mutation + commands | nothing (package-private) |
| `animation.go` | flash fade + continuous shimmer | nothing |
| `render.go` | row rendering | nothing |

Public surface is unchanged. Callers import `components/picker` and get `Picker`, `Item`, `Bind`, and the message types. All existing call sites compile after a single `import` rename — no other call-site changes.

### 4. Theme additions

One new style helper in `internal/tui/theme/theme.go`:

```go
// ShimmerGlowAt returns the foreground style for a row character at
// distance `d` from the shimmer head. d=0 is the brightest center;
// d=±1 are the shoulders; |d|≥2 falls back to `base`.
func ShimmerGlowAt(d int, base lipgloss.Style) lipgloss.Style
```

Implementation: a small switch on `abs(d)` returning the appropriate style. Keeps the brightness curve owned by the theme, not scattered across the picker.

### 5. Test

`internal/tui/components/picker/render_test.go`:

```go
func TestRowRenderRuneSafeAcrossShimmerPositions(t *testing.T) {
    // Build a row with a multi-byte char: "Pods · logs".
    // For every shimmer position 0..len(runes), assert:
    //   1. renderRow output is valid UTF-8 (utf8.ValidString).
    //   2. The visible width (ANSI-stripped) equals expected cell count.
    //   3. The output contains no '?' or '�' replacement characters.
}
```

This is the one test the refactor adds. Existing screens that wrap the picker aren't tested at the bubbletea level — that's existing scope.

## Migration

One commit. No incremental "live alongside the old file" phase — the move is mechanical and the public API is preserved by a same-package directory swap. The old `picker.go` file is deleted in the same commit.

Existing callers (`main_menu.go`, `pods_screen.go`, etc.) need exactly one import change:

```diff
-import "github.com/Tyka95/kube-kit/internal/tui/components"
+import "github.com/Tyka95/kube-kit/internal/tui/components/picker"
```

…and references like `components.Picker` become `picker.Picker`. A find-and-replace across `internal/tui/`. Roughly 12-15 files touched but each change is one identifier swap.

## Error handling

Render is total: every input produces a non-empty string. There are no error paths. The shimmer fallbacks (cold-cell style) cover degenerate cases like Width=0 or empty Items.

## Risks

- **Per-rune style-call overhead.** Mitigated by the cell-count budget being tiny (~800/render worst case).
- **Find-and-replace import rename misses a call site.** Mitigated by Go's compiler — anything missed is a build error.
- **Bold-on-bold doesn't visually pop** in some terminals (looks the same as plain bold). Mitigated by also raising the foreground color brightness at the head, not relying on weight alone.

## Out of scope

- Picker keybinding redesign (`:` command palette polish, `?` help overlay layout).
- Replacing `bubbles/list` — we don't use it.
- Animating selection-move transitions across multiple visible rows (e.g. a sliding ripple). Out of scope for this commit; the flash fade is enough.
