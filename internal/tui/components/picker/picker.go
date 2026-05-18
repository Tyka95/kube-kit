// Package picker is the kubekit list-selection widget. It owns one screen's
// list of choices, the cursor / scroll / filter state, and the move-fade +
// resting-shimmer animations. Screens embed a Picker value and pass tea.Msg
// events through its Update method.
package picker

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// Animation: selection-move feedback. The newly-selected row fades from
// bright accent (frame 0) through 4 intermediate steps back to the settled
// SelectBG (frame 5). Each step ~30ms — total ~150ms — smooth to the eye.
const selectFlashFrameInterval = 30 * time.Millisecond
const selectFlashFrames = 5 // frames 1..5; frame 0 is the initial bright paint.

// pickerFlashTickMsg advances the multi-frame fade animation. The token
// guards against late-arriving ticks from a previous move.
type pickerFlashTickMsg struct{ token int }

// Continuous shimmer on the resting selected row. A 3-cell-wide brighter
// band bounces left↔right across the row, ~7 fps. Token-guarded so the
// shimmer stops as soon as selection moves.
const shimmerTickInterval = 130 * time.Millisecond
const shimmerWidth = 3

type pickerShimmerTickMsg struct{ token int }

// Picker is a Bubble Tea sub-model.
//
// Use New() to construct, Update() / View() like any tea.Model. The picker
// emits typed messages on its parent's Update:
//
//   PickerSelectedMsg{Value string}  — Enter / right-arrow
//   PickerActionMsg{Action string}   — a custom Bind fired
//   PickerCancelMsg                  — Esc / q / left-arrow
//   PickerFilterMsg{Value string}    — '/' filter submitted
//   PickerCommandMsg{Value string}   — ':' command submitted
//   PickerHelpMsg                    — '?'
type Picker struct {
	Title  string
	Items  []Item
	Binds  []Bind
	Width  int
	Height int

	cursor      int
	scroll      int
	filter      string
	filterMode  bool
	commandMode bool
	input       string

	visible []int // indexes into Items after filtering

	// Flash animation state on selection change. flashFrame counts up from 0
	// to selectFlashFrames (settled). Token monotonically increases per move
	// so a late-arriving tick from an earlier move is ignored.
	flashFrame int
	flashToken int

	// Continuous shimmer state. shimmerPos / shimmerDir drive a small
	// brighter band bouncing across the selected row. shimmerToken matches
	// flashToken at start so move-events invalidate any in-flight shimmer.
	shimmerPos   int
	shimmerDir   int // +1 or -1
	shimmerToken int

	// initialized is true after the first Update arrives. Used to kick off
	// the initial shimmer on the resting cursor before the user moves it.
	initialized bool
}

// PickerSelectedMsg fires on Enter / right-arrow.
type PickerSelectedMsg struct{ Value string }

// PickerActionMsg fires when a custom Bind key is hit.
type PickerActionMsg struct{ Action string }

// PickerCancelMsg fires on Esc / q / left-arrow with no active filter.
type PickerCancelMsg struct{}

// PickerHelpMsg fires on '?'.
type PickerHelpMsg struct{}

// PickerCommandMsg fires when a ':' command is submitted.
type PickerCommandMsg struct{ Value string }

// New constructs an empty picker. Title is shown via the breadcrumb, not in
// the body.
func New(title string, items []Item, binds []Bind) Picker {
	p := Picker{Title: title, Items: items, Binds: binds}
	p.recomputeVisible()
	return p
}

// SetSize updates the picker's available area.
func (p *Picker) SetSize(width, height int) { p.Width = width; p.Height = height }

// Init is required by tea.Model.
func (p Picker) Init() tea.Cmd { return nil }

// Update handles key presses + animation ticks.
func (p Picker) Update(msg tea.Msg) (Picker, tea.Cmd) {
	// First message ever — start the shimmer so the resting cursor has motion
	// even before the user moves it. p.flashFrame stays at the settled value
	// so we skip the fade animation here.
	if !p.initialized {
		p.initialized = true
		p.flashFrame = selectFlashFrames // already settled
		shimmerCmd := p.startShimmer()
		// Recurse to let the rest of the switch handle this message.
		next, msgCmd := p.Update(msg)
		if shimmerCmd == nil {
			return next, msgCmd
		}
		if msgCmd == nil {
			return next, shimmerCmd
		}
		return next, tea.Batch(shimmerCmd, msgCmd)
	}

	// Flash-fade tick: advance the frame counter and schedule the next tick
	// while frames remain. When the fade settles, kick off the continuous
	// shimmer on the resting row.
	if tick, ok := msg.(pickerFlashTickMsg); ok {
		if tick.token != p.flashToken {
			return p, nil
		}
		if p.flashFrame < selectFlashFrames {
			p.flashFrame++
			if p.flashFrame < selectFlashFrames {
				tok := p.flashToken
				return p, tea.Tick(selectFlashFrameInterval, func(time.Time) tea.Msg {
					return pickerFlashTickMsg{token: tok}
				})
			}
			// Fade just finished → start the shimmer on this row.
			return p, p.startShimmer()
		}
		return p, nil
	}

	// Continuous shimmer tick: advance position, bounce at edges, schedule
	// the next tick. A stale token (from a prior selection) silently drops.
	if tick, ok := msg.(pickerShimmerTickMsg); ok {
		if tick.token != p.shimmerToken {
			return p, nil
		}
		// Compute row width once based on the longest visible item.
		maxCol := p.shimmerRangeMax()
		if maxCol < shimmerWidth {
			return p, nil
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
		})
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		return p.handleKey(km)
	}
	return p, nil
}

// flash starts a multi-frame selection fade. Frame 0 is bright accent;
// each tick advances toward the settled SelectBG. The token guards
// against ticks from a previous (now-stale) move. Bumping flashToken
// also invalidates the continuous shimmer on the previous row.
func (p *Picker) flash() tea.Cmd {
	p.flashToken++
	p.flashFrame = 0
	// Invalidate any in-flight shimmer from the previous selection.
	p.shimmerToken++
	p.shimmerPos = 0
	p.shimmerDir = 1
	tok := p.flashToken
	return tea.Tick(selectFlashFrameInterval, func(time.Time) tea.Msg {
		return pickerFlashTickMsg{token: tok}
	})
}

// startShimmer kicks off the continuous shimmer cycle on the now-resting
// selected row. Called when the flash-fade settles.
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
// to — roughly the width of the longest visible label + detail + meta. The
// 2-char lead-in is included so the shimmer can reach the right edge of the
// row, not just the text.
func (p Picker) shimmerRangeMax() int {
	if p.Width == 0 {
		return 0
	}
	// 80% of the visible row width — leaves a small idle margin at the
	// edges so the band doesn't park against a border.
	return (p.Width * 4) / 5
}

// View renders the picker body.
func (p Picker) View() string {
	if p.Width == 0 || p.Height == 0 {
		return ""
	}

	var b strings.Builder

	// Filter/command prompt row when active.
	if p.filterMode {
		b.WriteString(" " + theme.FooterKey.Render("/") + " " + theme.Body.Render(p.input))
		b.WriteString("\n")
	} else if p.commandMode {
		b.WriteString(" " + theme.FooterKey.Render(":") + " " + theme.Body.Render(p.input))
		b.WriteString("\n")
	} else if p.filter != "" {
		b.WriteString(" " + theme.FooterKey.Render("/") + " " + theme.Body.Render(p.filter))
		b.WriteString("\n")
	}

	// Empty list.
	if len(p.visible) == 0 {
		b.WriteString("  " + theme.Dim.Render("no matches"))
		return b.String()
	}

	// Measure column widths from visible items, capped.
	maxLbl, maxDet, maxMeta := 0, 0, 0
	for _, idx := range p.visible {
		it := p.Items[idx]
		if w := lipglossWidth(it.Label); w > maxLbl {
			maxLbl = w
		}
		if w := lipglossWidth(it.Detail); w > maxDet {
			maxDet = w
		}
		if w := lipglossWidth(it.Meta); w > maxMeta {
			maxMeta = w
		}
	}
	if maxLbl > 40 {
		maxLbl = 40
	}
	if maxDet > 60 {
		maxDet = 60
	}
	if maxMeta > 24 {
		maxMeta = 24
	}

	body := p.bodyRows()
	for vis := 0; vis < body; vis++ {
		idx := p.scroll + vis
		if idx >= len(p.visible) {
			break
		}
		it := p.Items[p.visible[idx]]
		isSel := idx == p.cursor

		label := padRight(truncate(it.Label, maxLbl), maxLbl)
		detail := padRight(truncate(it.Detail, maxDet), maxDet)
		meta := padLeft(truncate(it.Meta, maxMeta), maxMeta)

		var row string
		if isSel {
			// Selected row layout:
			//   █ <label>   <detail>   <meta>
			// The leading '█' is a full-block in accent fg painted on the
			// selection bg, so it reads as a bright vertical stripe at the
			// left edge of an unmistakably highlighted row. No floating
			// glyphs — the bar IS part of the bg block, so it can't render
			// disconnected if the user's font drops a rare codepoint.
			plain := " " + label + "   " + detail
			if it.Meta != "" {
				plain += "   " + meta
			}
			// Pad to (Width - 1) to leave room for the leading stripe column.
			plain = padRight(plain, p.Width-1)

			baseBG := theme.SelectionBGAt(p.flashFrame, selectFlashFrames)
			rendered := baseBG.Render(plain)

			if p.flashFrame >= selectFlashFrames {
				// Overlay the shimmer band only after the flash settles.
				width := len(plain)
				start := p.shimmerPos - shimmerWidth/2
				if start < 0 {
					start = 0
				}
				end := start + shimmerWidth
				if end > width {
					end = width
				}
				if end > start {
					bandStyle := lipgloss.NewStyle().
						Background(theme.SelectFlashBG).
						Foreground(theme.Primary).
						Bold(true)
					rendered = baseBG.Render(plain[:start]) +
						bandStyle.Render(plain[start:end]) +
						baseBG.Render(plain[end:])
				}
			}

			// Leading stripe — accent fg on the same selection bg so it
			// joins the row block visually. '█' is U+2588, in any
			// Unicode-capable font.
			stripe := theme.LeftStripe.Render("█")
			row = stripe + rendered
		} else {
			// Unselected row: 2-space lead-in matching the marker width so
			// the columns line up between selected and unselected rows.
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
		b.WriteString(row)
		b.WriteString("\n")
	}

	return b.String()
}

// lipglossWidth returns the printable cell width of a styled string by
// stripping ANSI sequences manually. Duplicated from components/footer.go
// because picker is now a separate package; the helper is too tiny to
// justify a shared "stringwidth" package.
func lipglossWidth(s string) int {
	n := 0
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
		n++
	}
	return n
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if lipglossWidth(s) <= max {
		return s
	}
	r := []rune(s)
	if max > len(r) {
		max = len(r)
	}
	return string(r[:max])
}

func padRight(s string, w int) string {
	gap := w - lipglossWidth(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

func padLeft(s string, w int) string {
	gap := w - lipglossWidth(s)
	if gap <= 0 {
		return s
	}
	return strings.Repeat(" ", gap) + s
}
