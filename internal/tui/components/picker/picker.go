// Package picker is the kubekit list-selection widget. It owns one screen's
// list of choices, the cursor / scroll / filter state, and the move-fade +
// resting-shimmer animations. Screens embed a Picker value and pass tea.Msg
// events through its Update method.
package picker

import (
	tea "github.com/charmbracelet/bubbletea"
)

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

