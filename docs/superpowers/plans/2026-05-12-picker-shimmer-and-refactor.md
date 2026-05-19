# Picker — Shimmer Fix + Refactor — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the UTF-8 corruption in the selected-row shimmer and split the 450-line `picker.go` into five single-responsibility files, with a targeted unit test that proves rune safety at every shimmer position.

**Architecture:** Move `internal/tui/components/picker.go` → `internal/tui/components/picker/` package directory. Split into `picker.go` / `state.go` / `input.go` / `animation.go` / `render.go`. Replace byte-level slicing in the row renderer with a `[]rune` walk that paints each character with a per-cell foreground style derived from the shimmer head position. Background of the selected row stays uniform.

**Tech Stack:** Go 1.22+, Bubble Tea 1.2, Lipgloss 1.0. `go test` for verification.

**Working directory:** `/Users/constantinsurdu/Repos/Personal/kube-kit`. Reference spec: [docs/superpowers/specs/2026-05-12-picker-shimmer-and-refactor.md](../specs/2026-05-12-picker-shimmer-and-refactor.md).

**Conventional commits.** No `Co-Authored-By` trailer.

---

## File Structure

After this plan completes:

```
internal/tui/components/
├── footer.go            unchanged
├── header.go            unchanged
├── spinner.go           unchanged
└── picker/              NEW directory, package picker
    ├── picker.go        public Picker type, Init/Update/View glue
    ├── state.go         Item, Bind, Position, filter, recompute
    ├── input.go         key dispatch (pure key→state+cmd)
    ├── animation.go     flash + shimmer tick handlers
    ├── render.go        rune-safe row paint + glow overlay
    └── render_test.go   rune-safety regression test

internal/tui/theme/theme.go    +1 helper: ShimmerGlowAt
```

Callers (`internal/tui/*.go` — main_menu, pods_screen, etc.) update one import to point at the new sub-package and replace `components.Picker` etc. with `picker.Picker`.

---

## Task 1: Add `theme.ShimmerGlowAt` helper

**Files:**
- Modify: `internal/tui/theme/theme.go`
- Create: `internal/tui/theme/theme_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/theme/theme_test.go`:

```go
package theme

import (
	"strings"
	"testing"
)

func TestShimmerGlowAtVaryingDistance(t *testing.T) {
	base := Dim
	cases := []struct {
		name string
		d    int
	}{
		{"head", 0},
		{"shoulder-pos", 1},
		{"shoulder-neg", -1},
		{"cold-far", 5},
		{"cold-neg", -5},
	}
	rendered := make(map[string]string, len(cases))
	for _, c := range cases {
		rendered[c.name] = ShimmerGlowAt(c.d, base).Render("X")
	}

	// Head must differ from a far/cold cell.
	if rendered["head"] == rendered["cold-far"] {
		t.Errorf("ShimmerGlowAt(0) and ShimmerGlowAt(5) produced identical output: %q", rendered["head"])
	}
	// Shoulders must differ from the head AND from cold cells (between-state).
	if rendered["shoulder-pos"] == rendered["head"] {
		t.Errorf("ShimmerGlowAt(1) and ShimmerGlowAt(0) produced identical output; the shoulder should be distinguishable")
	}
	if rendered["shoulder-pos"] == rendered["cold-far"] {
		t.Errorf("ShimmerGlowAt(1) and ShimmerGlowAt(5) produced identical output")
	}
	// Far-cold cells should fall back to the base style regardless of sign.
	if rendered["cold-far"] != rendered["cold-neg"] {
		t.Errorf("ShimmerGlowAt(5) and ShimmerGlowAt(-5) differ; both should fall back to base")
	}
	// Sanity: every rendering contains the literal payload, not garbled.
	for name, out := range rendered {
		if !strings.Contains(out, "X") {
			t.Errorf("ShimmerGlowAt(%s) dropped the payload: %q", name, out)
		}
	}
}
```

- [ ] **Step 2: Verify the test fails (function not defined yet)**

Run:
```bash
cd /Users/constantinsurdu/Repos/Personal/kube-kit
go test ./internal/tui/theme/...
```
Expected output contains `undefined: ShimmerGlowAt`. Confirm before continuing.

- [ ] **Step 3: Implement `ShimmerGlowAt`**

Append to `internal/tui/theme/theme.go` (after the existing `SelectionBGAt` block):

```go
// ShimmerGlowAt returns the foreground style for a row character at signed
// distance `d` from the shimmer head:
//   d == 0       → bright primary + bold (the moving "head" of the glow)
//   |d| == 1     → shoulder blend (bold, accent-tinted)
//   |d| >= 2     → returns base unchanged (cold cell — no glow effect)
//
// The selection background is owned by the row renderer; this helper only
// shapes the foreground. Same input always produces the same Style — pure.
func ShimmerGlowAt(d int, base lipgloss.Style) lipgloss.Style {
	abs := d
	if abs < 0 {
		abs = -abs
	}
	switch abs {
	case 0:
		return lipgloss.NewStyle().Foreground(Primary).Bold(true)
	case 1:
		return lipgloss.NewStyle().Foreground(Accent).Bold(true)
	default:
		return base
	}
}
```

- [ ] **Step 4: Verify the test passes**

```bash
go test ./internal/tui/theme/...
```
Expected: `ok   github.com/Tyka95/kube-kit/internal/tui/theme   <duration>`.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/theme/
git commit -m "feat(theme): ShimmerGlowAt foreground helper for picker row glow"
```

---

## Task 2: Move `picker.go` into its own sub-package

**Files:**
- Create directory: `internal/tui/components/picker/`
- Move: `internal/tui/components/picker.go` → `internal/tui/components/picker/picker.go`
- Modify (package decl): `internal/tui/components/picker/picker.go`
- Modify (imports + references): every screen file under `internal/tui/` that uses the picker

- [ ] **Step 1: Make the directory and move the file**

```bash
cd /Users/constantinsurdu/Repos/Personal/kube-kit
mkdir -p internal/tui/components/picker
git mv internal/tui/components/picker.go internal/tui/components/picker/picker.go
```

- [ ] **Step 2: Rename the package declaration**

In `internal/tui/components/picker/picker.go`, change line 1 from:
```go
package components
```
to:
```go
package picker
```

- [ ] **Step 3: Find every caller and rewrite the import + identifiers**

```bash
grep -rln 'components\.\(Picker\|Item\|Bind\|PickerSelectedMsg\|PickerCancelMsg\|PickerActionMsg\|PickerCommandMsg\|PickerHelpMsg\)' internal/ cmd/ 2>/dev/null
```

For each file in the output, perform exactly two changes:

1. Add `"github.com/Tyka95/kube-kit/internal/tui/components/picker"` to its import block. Keep the existing `"github.com/Tyka95/kube-kit/internal/tui/components"` import only if the file still uses Header / Footer / Spinner; remove it otherwise.

2. Replace identifiers via:
```bash
perl -i -pe '
  s/\bcomponents\.Picker\b/picker.Picker/g;
  s/\bcomponents\.Item\b/picker.Item/g;
  s/\bcomponents\.Bind\b/picker.Bind/g;
  s/\bcomponents\.PickerSelectedMsg\b/picker.PickerSelectedMsg/g;
  s/\bcomponents\.PickerCancelMsg\b/picker.PickerCancelMsg/g;
  s/\bcomponents\.PickerActionMsg\b/picker.PickerActionMsg/g;
  s/\bcomponents\.PickerCommandMsg\b/picker.PickerCommandMsg/g;
  s/\bcomponents\.PickerHelpMsg\b/picker.PickerHelpMsg/g;
  s/\bcomponents\.New\(/picker.New(/g;
' $(grep -rln 'components\.\(Picker\|Item\|Bind\|PickerSelectedMsg\|PickerCancelMsg\|PickerActionMsg\|PickerCommandMsg\|PickerHelpMsg\|New\)' internal/ cmd/ 2>/dev/null)
```

- [ ] **Step 4: Verify build still passes**

```bash
go build ./...
```

If a file errors on an unused `components` import, remove it manually. Re-run until clean.

- [ ] **Step 5: Verify all tests still pass**

```bash
go test ./...
```
Expected: all green (no behavior changed yet).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(picker): move into internal/tui/components/picker sub-package

No behavior change. Drops one cross-call-site identifier prefix
(components.Picker → picker.Picker) and sets up the directory layout
for the upcoming file split."
```

---

## Task 3: Extract `state.go`

**Files:**
- Modify: `internal/tui/components/picker/picker.go`
- Create: `internal/tui/components/picker/state.go`

- [ ] **Step 1: Identify the state-model code in picker.go**

Open `internal/tui/components/picker/picker.go` and locate:
- `type Item struct { ... }` definition
- `type Bind struct { ... }` definition
- `func (p Picker) Position() (int, int)` method
- `func (p *Picker) recomputeVisible()` method

These are the pure-data + filter pieces moving to `state.go`. Note their line ranges.

- [ ] **Step 2: Create `state.go` with the extracted code**

Write `internal/tui/components/picker/state.go`:

```go
package picker

import "strings"

// Item is a row in the picker. Label is the value returned on selection;
// Detail and Meta are the two informational columns rendered next to it.
type Item struct {
	Label  string
	Detail string
	Meta   string
}

// Bind is a per-screen custom keybinding. e.g. {Key: "r", Action: "refresh"}
// → pressing 'r' emits PickerActionMsg{Action: "refresh"} for the parent
// screen to handle.
type Bind struct {
	Key    string
	Action string
}

// Position returns the 1-based position of the cursor within the currently
// visible (filtered) item list. Returns (0, 0) when the list is empty.
func (p Picker) Position() (int, int) {
	if len(p.visible) == 0 {
		return 0, 0
	}
	return p.cursor + 1, len(p.visible)
}

// recomputeVisible rebuilds the `visible` index slice from the current
// filter buffer. Matching is case-insensitive substring on Label or Detail.
func (p *Picker) recomputeVisible() {
	p.visible = p.visible[:0]
	if p.filter == "" {
		for i := range p.Items {
			p.visible = append(p.visible, i)
		}
		return
	}
	needle := strings.ToLower(p.filter)
	for i, it := range p.Items {
		if strings.Contains(strings.ToLower(it.Label), needle) ||
			strings.Contains(strings.ToLower(it.Detail), needle) {
			p.visible = append(p.visible, i)
		}
	}
}
```

- [ ] **Step 3: Delete the same code from `picker.go`**

In `internal/tui/components/picker/picker.go`, delete:
- The `Item` and `Bind` type declarations.
- The `(p Picker) Position()` method.
- The `(p *Picker) recomputeVisible()` method.
- Any `"strings"` import line that is no longer needed (the build error will flag this).

- [ ] **Step 4: Verify build + tests pass**

```bash
go build ./... && go test ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/tui/components/picker/
git commit -m "refactor(picker): extract Item/Bind/Position/filter into state.go"
```

---

## Task 4: Extract `input.go`

**Files:**
- Modify: `internal/tui/components/picker/picker.go`
- Create: `internal/tui/components/picker/input.go`

- [ ] **Step 1: Identify the key-dispatch code**

In `picker.go`, locate the `Update` method's `switch km.String()` branch — the section that handles `"up"`, `"down"`, `"enter"`, `"esc"`, `"q"`, `"left"`, `"right"`, `"?"`, `"/"`, `":"`, custom Binds, and the filter-mode input. Also note the helper `func (p Picker) updateInput(km tea.KeyMsg) (Picker, tea.Cmd)`.

- [ ] **Step 2: Create `input.go`**

Write `internal/tui/components/picker/input.go`:

```go
package picker

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleKey dispatches a tea.KeyMsg into state mutations + optional command.
// It is pure relative to time: no timers, no I/O, no rendering. The Picker
// receiver is by value so callers can decide whether to commit the result.
func (p Picker) handleKey(km tea.KeyMsg) (Picker, tea.Cmd) {
	if p.filterMode || p.commandMode {
		return p.handleInputModeKey(km)
	}

	switch km.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		} else if len(p.visible) > 0 {
			p.cursor = len(p.visible) - 1
		}
		p.adjustScroll()
		return p, p.flash()
	case "down", "j":
		if p.cursor < len(p.visible)-1 {
			p.cursor++
		} else {
			p.cursor = 0
		}
		p.adjustScroll()
		return p, p.flash()
	case "enter", "right", "l":
		if len(p.visible) == 0 {
			return p, nil
		}
		val := p.Items[p.visible[p.cursor]].Label
		return p, func() tea.Msg { return PickerSelectedMsg{Value: val} }
	case "esc", "q", "left", "h":
		return p, func() tea.Msg { return PickerCancelMsg{} }
	case "?":
		return p, func() tea.Msg { return PickerHelpMsg{} }
	case "/":
		p.filterMode = true
		p.input = p.filter
		return p, nil
	case ":":
		p.commandMode = true
		p.input = ""
		return p, nil
	}

	for _, b := range p.Binds {
		if b.Key == km.String() {
			action := b.Action
			return p, func() tea.Msg { return PickerActionMsg{Action: action} }
		}
	}
	return p, nil
}

// handleInputModeKey handles characters typed into the '/' filter or ':'
// command input mode.
func (p Picker) handleInputModeKey(km tea.KeyMsg) (Picker, tea.Cmd) {
	switch km.String() {
	case "esc":
		p.filterMode = false
		p.commandMode = false
		p.input = ""
		return p, nil
	case "enter":
		val := p.input
		wasCmd := p.commandMode
		p.filterMode = false
		p.commandMode = false
		if wasCmd {
			p.input = ""
			return p, func() tea.Msg { return PickerCommandMsg{Value: val} }
		}
		p.filter = val
		p.input = ""
		p.recomputeVisible()
		p.cursor = 0
		p.scroll = 0
		return p, nil
	case "backspace":
		if len(p.input) > 0 {
			p.input = p.input[:len(p.input)-1]
		}
		return p, nil
	}
	if len(km.Runes) > 0 {
		p.input += string(km.Runes)
	}
	return p, nil
}

// adjustScroll shifts the visible window so the cursor stays in view.
func (p *Picker) adjustScroll() {
	if p.cursor < p.scroll {
		p.scroll = p.cursor
	} else if p.cursor >= p.scroll+p.bodyRows() {
		p.scroll = p.cursor - p.bodyRows() + 1
	}
}

// bodyRows returns how many list rows fit between filter prompt (if any)
// and the bottom of the picker's reserved area.
func (p Picker) bodyRows() int {
	rows := p.Height
	if p.filterMode || p.commandMode || p.filter != "" {
		rows--
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}
```

- [ ] **Step 3: Replace the key-dispatch in `picker.go` Update with a call to `handleKey`**

In `internal/tui/components/picker/picker.go`'s `Update` method, after the animation-tick branches, replace the entire `tea.KeyMsg` handling section (including the inner `updateInput`, `adjustScroll`, `bodyRows`) with:

```go
	if km, ok := msg.(tea.KeyMsg); ok {
		return p.handleKey(km)
	}
	return p, nil
```

Also remove `updateInput`, `adjustScroll`, `bodyRows` from `picker.go` — they moved to `input.go`.

- [ ] **Step 4: Verify build + tests**

```bash
go build ./... && go test ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/tui/components/picker/
git commit -m "refactor(picker): extract key dispatch + filter-mode input into input.go"
```

---

## Task 5: Extract `animation.go`

**Files:**
- Modify: `internal/tui/components/picker/picker.go`
- Create: `internal/tui/components/picker/animation.go`

- [ ] **Step 1: Identify the animation code in `picker.go`**

Locate and note:
- Constants `selectFlashFrameInterval`, `selectFlashFrames`, `shimmerTickInterval`, `shimmerWidth`.
- Message types `pickerFlashTickMsg`, `pickerShimmerTickMsg`.
- Picker fields `flashFrame`, `flashToken`, `shimmerPos`, `shimmerDir`, `shimmerToken`, `initialized`.
- Methods `flash`, `startShimmer`, `shimmerRangeMax`.
- The animation-tick handler branches at the top of `Update`.

- [ ] **Step 2: Create `animation.go`**

Write `internal/tui/components/picker/animation.go`:

```go
package picker

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Selection-move feedback: the newly-selected row fades from bright accent
// (frame 0) through 4 intermediate steps back to the settled SelectBG
// (frame `selectFlashFrames`). Each step is ~30ms; total ~150ms.
const (
	selectFlashFrameInterval = 30 * time.Millisecond
	selectFlashFrames        = 5
)

// Continuous shimmer on the resting row: a brighter 3-cell band bouncing
// left↔right at ~7 fps. Width and tick interval are both stable constants;
// tweak in one place to retune the effect.
const (
	shimmerTickInterval = 130 * time.Millisecond
	shimmerWidth        = 3
)

// pickerFlashTickMsg advances the multi-frame flash fade. Token-guarded so a
// late-arriving tick from a previous move is ignored.
type pickerFlashTickMsg struct{ token int }

// pickerShimmerTickMsg advances the continuous shimmer position. Token-guarded
// so a selection change invalidates any in-flight shimmer.
type pickerShimmerTickMsg struct{ token int }

// handleTick advances any in-flight animation. Returns (newPicker, cmd, true)
// if it handled the message; (_, _, false) otherwise so Update can fall through
// to the input handler.
func (p Picker) handleTick(msg tea.Msg) (Picker, tea.Cmd, bool) {
	switch tick := msg.(type) {
	case pickerFlashTickMsg:
		return p.advanceFlash(tick.token)
	case pickerShimmerTickMsg:
		return p.advanceShimmer(tick.token)
	}
	return p, nil, false
}

func (p Picker) advanceFlash(token int) (Picker, tea.Cmd, bool) {
	if token != p.flashToken {
		return p, nil, true
	}
	if p.flashFrame < selectFlashFrames {
		p.flashFrame++
		if p.flashFrame < selectFlashFrames {
			tok := p.flashToken
			return p, tea.Tick(selectFlashFrameInterval, func(time.Time) tea.Msg {
				return pickerFlashTickMsg{token: tok}
			}), true
		}
		return p, p.startShimmer(), true
	}
	return p, nil, true
}

func (p Picker) advanceShimmer(token int) (Picker, tea.Cmd, bool) {
	if token != p.shimmerToken {
		return p, nil, true
	}
	maxCol := p.shimmerRangeMax()
	if maxCol < shimmerWidth {
		return p, nil, true
	}
	p.shimmerPos += p.shimmerDir
	if p.shimmerPos >= maxCol {
		p.shimmerPos = maxCol
		p.shimmerDir = -1
	} else if p.shimmerPos <= 0 {
		p.shimmerPos = 0
		p.shimmerDir = 1
	}
	tok := p.shimmerToken
	return p, tea.Tick(shimmerTickInterval, func(time.Time) tea.Msg {
		return pickerShimmerTickMsg{token: tok}
	}), true
}

// flash starts a new selection-move fade. Bumping flashToken also invalidates
// any in-flight shimmer from the previous row.
func (p *Picker) flash() tea.Cmd {
	p.flashToken++
	p.flashFrame = 0
	p.shimmerToken++
	p.shimmerPos = 0
	p.shimmerDir = 1
	tok := p.flashToken
	return tea.Tick(selectFlashFrameInterval, func(time.Time) tea.Msg {
		return pickerFlashTickMsg{token: tok}
	})
}

// startShimmer kicks off the continuous shimmer on the now-resting selected
// row. Called automatically when the fade settles.
func (p *Picker) startShimmer() tea.Cmd {
	p.shimmerToken++
	p.shimmerPos = 0
	p.shimmerDir = 1
	tok := p.shimmerToken
	return tea.Tick(shimmerTickInterval, func(time.Time) tea.Msg {
		return pickerShimmerTickMsg{token: tok}
	})
}

// shimmerRangeMax returns the rightmost column the shimmer head can travel
// to. 80% of the visible row width — leaves a small margin so the band
// doesn't park against the right edge.
func (p Picker) shimmerRangeMax() int {
	if p.Width == 0 {
		return 0
	}
	return (p.Width * 4) / 5
}
```

- [ ] **Step 3: Delete the same code from `picker.go`**

Remove from `picker.go`:
- The four animation constants.
- The two tick message types.
- The `flash`, `startShimmer`, `shimmerRangeMax` methods.
- The animation-tick branches at the top of `Update`.
- Any `"time"` import that becomes unused.

Replace the animation-tick handling in `Update` with a single delegation. The new top of `Update` should look like:

```go
func (p Picker) Update(msg tea.Msg) (Picker, tea.Cmd) {
	if !p.initialized {
		p.initialized = true
		p.flashFrame = selectFlashFrames
		shimmerCmd := p.startShimmer()
		next, msgCmd := p.Update(msg)
		switch {
		case shimmerCmd == nil:
			return next, msgCmd
		case msgCmd == nil:
			return next, shimmerCmd
		default:
			return next, tea.Batch(shimmerCmd, msgCmd)
		}
	}

	if np, cmd, handled := p.handleTick(msg); handled {
		return np, cmd
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		return p.handleKey(km)
	}
	return p, nil
}
```

- [ ] **Step 4: Verify build + tests**

```bash
go build ./... && go test ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/tui/components/picker/
git commit -m "refactor(picker): extract flash + shimmer ticks into animation.go"
```

---

## Task 6: Extract `render.go` (byte-slice behavior preserved)

**Files:**
- Modify: `internal/tui/components/picker/picker.go`
- Create: `internal/tui/components/picker/render.go`

This task moves the existing renderer verbatim. The rune-safety fix lands in Task 8 after the regression test in Task 7. Two commits = two reasons.

- [ ] **Step 1: Identify rendering code**

Locate in `picker.go`:
- `func (p Picker) View() string`
- The `lipglossWidth`, `truncate`, `padRight`, `padLeft` helpers.

- [ ] **Step 2: Create `render.go` and move the View + helpers verbatim**

Write `internal/tui/components/picker/render.go` with the EXACT current `View` body + the four helpers, plus the imports `lipgloss`, `strings`, and `internal/tui/theme`. Do not change any logic in this commit.

Skeleton (fill in the body identical to the current `View` and helpers):

```go
package picker

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// View renders the picker body — filter prompt (if active) + the visible
// item rows.
func (p Picker) View() string {
	// [paste the current View body here, unchanged]
}

func lipglossWidth(s string) int { /* paste current impl */ }
func truncate(s string, max int) string { /* paste current impl */ }
func padRight(s string, w int) string { /* paste current impl */ }
func padLeft(s string, w int) string { /* paste current impl */ }
```

- [ ] **Step 3: Delete the same code from `picker.go`**

Remove `View` and the four helpers from `picker.go`. Drop any imports that are now unused.

- [ ] **Step 4: Verify build + tests**

```bash
go build ./... && go test ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/tui/components/picker/
git commit -m "refactor(picker): extract View + width helpers into render.go (verbatim)"
```

---

## Task 7: Write the rune-safety regression test

**Files:**
- Create: `internal/tui/components/picker/render_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/components/picker/render_test.go`:

```go
package picker

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// stripANSI removes CSI escape sequences from s so callers can measure
// visible width or scan for replacement characters without false positives.
func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		if r == 0x1b {
			in = true
			continue
		}
		if in {
			if r == 'm' {
				in = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// TestRenderRowRuneSafeAcrossShimmerPositions builds a selected row containing
// a multi-byte UTF-8 character (`·` U+00B7, 2 bytes) and asserts the renderer
// produces valid UTF-8 output free of replacement glyphs at every possible
// shimmer head position. Catches the byte-slice bug that produces `??`.
func TestRenderRowRuneSafeAcrossShimmerPositions(t *testing.T) {
	items := []Item{
		{Label: "Pods", Detail: "list · logs · shell · inspect"},
	}
	p := New("test", items, nil)
	p.SetSize(80, 10)
	// Pin the picker into the resting selected state so the shimmer code path
	// actually executes (it only kicks in once the flash fade has settled).
	p.flashFrame = selectFlashFrames

	itemPlainLen := len("  Pods   list · logs · shell · inspect")
	for pos := 0; pos <= itemPlainLen+5; pos++ {
		p.shimmerPos = pos
		out := p.View()

		if !utf8.ValidString(out) {
			t.Errorf("shimmerPos=%d produced invalid UTF-8 output: %q", pos, out)
		}
		visible := stripANSI(out)
		if strings.ContainsRune(visible, '�') {
			t.Errorf("shimmerPos=%d produced a U+FFFD replacement character: %q", pos, visible)
		}
		if strings.Contains(visible, "??") {
			t.Errorf("shimmerPos=%d produced literal '??': %q", pos, visible)
		}
	}
}
```

- [ ] **Step 2: Verify the test fails on the current byte-slice implementation**

```bash
cd /Users/constantinsurdu/Repos/Personal/kube-kit
go test ./internal/tui/components/picker/...
```

Expected: the test fails for at least one `shimmerPos` value with either a `?` substring or an invalid-UTF-8 error. Capture the failure output to confirm — this is the bug we're about to fix.

If the test passes on byte-slice code, the test is too lenient. Double-check the row labels include `·` (U+00B7), not the ASCII `·` look-alike.

- [ ] **Step 3: Commit the failing test**

```bash
git add internal/tui/components/picker/render_test.go
git commit -m "test(picker): rune-safety regression at every shimmer position"
```

Yes, committing a failing test on purpose — Task 8 fixes it and that commit demonstrates the fix.

---

## Task 8: Rewrite `render.go` rune-safe with foreground glow

**Files:**
- Modify: `internal/tui/components/picker/render.go`

- [ ] **Step 1: Replace the selected-row paint block in `render.go`'s `View`**

Locate the section of `View` that handles `if isSel { ... }` — the block that builds `plain`, calls `padRight`, then slices `plain[start:end]` to paint the shimmer band. Replace it with the rune-safe version:

```go
		var row string
		if isSel {
			// Selected row layout:
			//   █ <label>   <detail>   <meta>
			// Render rune-by-rune so multi-byte UTF-8 characters (like `·`)
			// never get sliced in half. The shimmer is foreground-only:
			// per-cell brightness derived from distance to shimmerPos.
			plain := " " + label + "   " + detail
			if it.Meta != "" {
				plain += "   " + meta
			}
			plain = padRight(plain, p.Width-1) // -1 leaves room for left stripe

			baseBG := theme.SelectionBGAt(p.flashFrame, selectFlashFrames)
			labelEnd := 1 + lipglossWidth(label) // first char after left padding + label runes
			runes := []rune(plain)

			var b strings.Builder
			for col, r := range runes {
				// Base fg: bold primary in the label zone, muted elsewhere.
				var base lipgloss.Style
				if col <= labelEnd {
					base = theme.ListLabel
				} else {
					base = theme.ListDetail
				}
				// Apply shimmer glow only after the flash fade has settled.
				var fg lipgloss.Style
				if p.flashFrame >= selectFlashFrames {
					fg = theme.ShimmerGlowAt(col-p.shimmerPos, base)
				} else {
					fg = base
				}
				// Compose: foreground style + selection bg. Background style
				// wins on the bg axis; foreground style on the fg axis.
				cell := fg.Inherit(baseBG).Render(string(r))
				b.WriteString(cell)
			}

			stripe := theme.LeftStripe.Render("█")
			row = stripe + b.String()
		} else {
			// Unselected row: per-column color hierarchy (no shimmer math).
			labelCol := theme.ListLabel.Render(label)
			detailCol := theme.ListDetail.Render(detail)
			metaCol := theme.ListMeta.Render(meta)
			row = "  " + labelCol + "   " + detailCol
			if it.Meta != "" {
				row += "   " + metaCol
			}
			row = padRight(row, p.Width)
			row = lipgloss.NewStyle().MaxWidth(p.Width).Render(row)
		}
```

The key differences from the previous implementation:
- `[]rune(plain)` walk replaces byte slicing — UTF-8 safe by construction.
- Each rune renders independently with its own foreground style.
- `theme.ShimmerGlowAt(col - p.shimmerPos, base)` returns the per-cell foreground; the selection bg is inherited via `fg.Inherit(baseBG)`.

- [ ] **Step 2: Verify the rune-safety test now passes**

```bash
go test ./internal/tui/components/picker/...
```
Expected: `ok   github.com/Tyka95/kube-kit/internal/tui/components/picker   <duration>`.

- [ ] **Step 3: Verify the whole test suite + build**

```bash
go build ./... && go test ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/tui/components/picker/render.go
git commit -m "fix(picker): rune-safe row rendering with foreground-only shimmer glow

The previous byte-slice approach (plain[start:end]) cut through multi-byte
UTF-8 characters like '·' (U+00B7, 2 bytes), producing '??' replacement
glyphs in the row. Each rune now renders independently with its own
foreground style derived from distance to the shimmer head — no slicing
into byte buffers possible.

Visually: the shimmer changes only the character foreground (bright at
the head, accent-tinted at the shoulders, base color elsewhere) instead
of painting its own background block. The row's selection bg stays
uniform — reads as a light moving across the text rather than an
overlay square."
```

---

## Task 9: End-to-end verification

- [ ] **Step 1: Build the binary**

```bash
cd /Users/constantinsurdu/Repos/Personal/kube-kit
make build
```
Expected: `bin/kubekit` exists, no compile errors.

- [ ] **Step 2: Run the full test suite**

```bash
go test ./...
```
Expected: every package shows `ok`. No `FAIL` lines.

- [ ] **Step 3: Static check the picker file size**

```bash
wc -l internal/tui/components/picker/*.go
```
Expected: each file under 200 lines.

- [ ] **Step 4: Audit for project-specific leakage**

```bash
grep -rnE 'soccer|oddsforge|sportcontext|production-eks|staging-eks|963758802620|072092343946' \
  --include='*.go' internal/ cmd/ 2>/dev/null
```
Expected: no output.

- [ ] **Step 5: Manual smoke run**

```bash
./bin/kubekit
```
Walk through:
1. Main menu — arrow up/down, watch the flash fade, then the shimmer kicks in on the resting row.
2. With cursor parked, confirm no `?` glyphs anywhere — the `·` separators stay clean as the shimmer passes over them.
3. Press `Pods` → arrow through pod actions. Same behavior.
4. Press `q` → back to main. Menu reads correctly.
5. Press `:q` to quit.

- [ ] **Step 6: Final reconciliation commit (only if any cleanup was needed)**

If Step 5 surfaced anything, fix it and:

```bash
git add -A
git commit -m "fix(picker): cleanup after manual smoke pass"
```

If clean, skip this step.

---

## Self-review

**Spec coverage:**
- Goal 1 (foreground glow) → Task 1 (theme helper) + Task 8 (rewrite renderer to use it).
- Goal 2 (rune safety) → Task 7 (regression test) + Task 8 (rewrite).
- Goal 3 (focused files) → Tasks 2-6 (move + split into 5 files).
- Goal 4 (one targeted test) → Task 7.

**Placeholder scan:** Task 6 step 2 says "paste the current View body here, unchanged" — that is intentional (mechanical move) and the engineer has the file content to copy verbatim. No "TBD" / "handle edge cases" / "TODO later" anywhere else.

**Type / name consistency:**
- `ShimmerGlowAt(d int, base lipgloss.Style) lipgloss.Style` — Task 1 definition matches Task 8 call.
- `selectFlashFrames` constant — used in Tasks 5, 7, 8 consistently.
- `flashFrame`, `shimmerPos`, `shimmerToken` field names — defined in `Picker` struct (existing), referenced in Tasks 5, 7, 8 with same spelling.
- `theme.SelectionBGAt`, `theme.LeftStripe`, `theme.ListLabel`, `theme.ListDetail`, `theme.ListMeta` — all already exist; the plan only adds `ShimmerGlowAt`.
- Method names: `handleKey`, `handleTick`, `advanceFlash`, `advanceShimmer`, `flash`, `startShimmer`, `shimmerRangeMax`, `bodyRows`, `adjustScroll`, `recomputeVisible` — used consistently across Tasks 3, 4, 5.

Plan looks complete.
