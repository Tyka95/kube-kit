package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui/components/picker"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// awsActionDoneMsg is dispatched when an async AWS action completes.
type awsActionDoneMsg struct {
	Status string
}

// AWSScreen is the AWS submenu screen.
type AWSScreen struct {
	picker picker.Picker
	// status text shown above the picker while an action is in flight
	status string
}

// NewAWSScreen constructs the AWS submenu screen.
func NewAWSScreen() *AWSScreen {
	items := []picker.Item{
		{Label: "SSO Login", Detail: "list profiles · aws sso login"},
		{Label: "EKS Connect", Detail: "configure kubectl context"},
		{Label: "S3 Buckets", Detail: "browse buckets"},
	}
	return &AWSScreen{
		picker: picker.New("AWS", items, nil),
	}
}

// Init returns nil — no async work on entry.
func (s *AWSScreen) Init() tea.Cmd { return nil }

// Breadcrumb returns the label pushed onto the breadcrumb trail.
func (s *AWSScreen) Breadcrumb() string { return "AWS" }

// KeyHints returns the per-screen contextual hints.
func (s *AWSScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{
		{Key: "⏎", Action: "run"},
		{Key: "?", Action: "help"},
	}
}

// Position satisfies PositionProvider so the footer can show n/total.
func (s *AWSScreen) Position() (int, int) { return s.picker.Position() }

// Update routes messages to the picker and handles picker events.
func (s *AWSScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		s.picker.SetSize(m.Width, pickerBodyHeight(m.Height))
		return s, nil

	case picker.PickerSelectedMsg:
		switch m.Value {
		case "SSO Login":
			// Push the SSO profile picker. Picking a profile then pushes
			// the actual SSO login screen — the auto-pick that used to
			// silently log into the first profile (almost always `stage`)
			// is gone.
			s.status = ""
			app.Push(NewSSOProfilePickerScreen())
			return s, nil
		case "EKS Connect":
			s.status = "EKS Connect: not yet implemented"
			return s, nil
		case "S3 Buckets":
			s.status = "S3 Buckets: not yet implemented"
			return s, nil
		}
		return s, nil

	case picker.PickerCancelMsg:
		// Self-pop — signal the app to pop this screen.
		return nil, nil

	case awsActionDoneMsg:
		s.status = m.Status
		return s, nil
	}

	var cmd tea.Cmd
	s.picker, cmd = s.picker.Update(msg)
	return s, cmd
}

// View renders the AWS screen body.
func (s *AWSScreen) View(app *App) string {
	s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))
	if s.status != "" {
		return theme.InfoCallout("info", s.status) + "\n\n" + s.picker.View()
	}
	return s.picker.View()
}

