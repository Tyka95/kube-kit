package picker

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Selection-move feedback: the newly-selected row fades from bright accent
// (frame 0) through 4 intermediate steps back to the settled SelectBG
// (frame selectFlashFrames). Each step is ~30ms; total ~150ms.
const (
	selectFlashFrameInterval = 30 * time.Millisecond
	selectFlashFrames        = 5
)

// Continuous shimmer on the resting row: a brighter 3-cell band bouncing
// left↔right at ~7 fps. Tweak in one place to retune the effect.
const (
	shimmerTickInterval = 130 * time.Millisecond
	shimmerWidth        = 3
)

// pickerFlashTickMsg advances the multi-frame flash fade. Token-guarded so
// a late-arriving tick from a previous move is ignored.
type pickerFlashTickMsg struct{ token int }

// pickerShimmerTickMsg advances the continuous shimmer position. Token-guarded
// so a selection change invalidates any in-flight shimmer.
type pickerShimmerTickMsg struct{ token int }

// handleTick advances any in-flight animation. Returns (newPicker, cmd, true)
// if it handled the message; (_, _, false) if Update should fall through to
// the key handler.
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
// to — 80% of the visible row width so the band doesn't park against the
// right edge.
func (p Picker) shimmerRangeMax() int {
	if p.Width == 0 {
		return 0
	}
	return (p.Width * 4) / 5
}
