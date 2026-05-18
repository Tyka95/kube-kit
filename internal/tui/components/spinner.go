package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// Spinner is a tiny braille spinner that screens can embed in their View.
// Use it for loading indicators that should feel responsive.
type Spinner struct {
	frames []string
	idx    int
	active bool
	token  int
}

// SpinnerTickMsg is the periodic tick that advances the spinner.
type SpinnerTickMsg struct{ token int }

const spinnerInterval = 90 * time.Millisecond

// NewSpinner constructs a fresh spinner (not yet running).
func NewSpinner() Spinner {
	return Spinner{frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}}
}

// Start returns a Cmd that schedules the first tick. Idempotent — calling
// Start while already running just refreshes the token (canceling stale ticks).
func (s *Spinner) Start() tea.Cmd {
	s.active = true
	s.token++
	return s.scheduleTick()
}

// Stop halts the spinner. Late ticks with stale tokens are ignored.
func (s *Spinner) Stop() {
	s.active = false
	s.token++
}

// Update advances the spinner on its own tick message. Returns a Cmd to
// schedule the next tick when still active.
func (s *Spinner) Update(msg tea.Msg) (tea.Cmd, bool) {
	tick, ok := msg.(SpinnerTickMsg)
	if !ok {
		return nil, false
	}
	if !s.active || tick.token != s.token {
		return nil, true
	}
	s.idx = (s.idx + 1) % len(s.frames)
	return s.scheduleTick(), true
}

// View returns the current spinner frame (in accent color) or empty string
// when stopped.
func (s Spinner) View() string {
	if !s.active {
		return ""
	}
	return theme.ListAccentBar.Render(s.frames[s.idx])
}

func (s *Spinner) scheduleTick() tea.Cmd {
	tok := s.token
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg {
		return SpinnerTickMsg{token: tok}
	})
}
