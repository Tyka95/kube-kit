package tui

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/awssession"
	"github.com/Tyka95/kube-kit/internal/tui/components"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// awsActionDoneMsg is dispatched when an async AWS action completes.
type awsActionDoneMsg struct {
	Status string
}

// AWSScreen is the AWS submenu screen.
type AWSScreen struct {
	picker components.Picker
	// status text shown above the picker while an action is in flight
	status string
}

// NewAWSScreen constructs the AWS submenu screen.
func NewAWSScreen() *AWSScreen {
	items := []components.Item{
		{Label: "SSO Login", Detail: "list profiles · aws sso login"},
		{Label: "EKS Connect", Detail: "configure kubectl context"},
		{Label: "S3 Buckets", Detail: "browse buckets"},
	}
	return &AWSScreen{
		picker: components.New("AWS", items, nil),
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

	case components.PickerSelectedMsg:
		switch m.Value {
		case "SSO Login":
			return s, func() tea.Msg {
				status := pickProfileAndLogin()
				return awsActionDoneMsg{Status: status}
			}
		case "EKS Connect":
			s.status = "EKS Connect: not yet implemented"
			return s, nil
		case "S3 Buckets":
			s.status = "S3 Buckets: not yet implemented"
			return s, nil
		}
		return s, nil

	case components.PickerCancelMsg:
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
		return theme.Dim.Render(s.status) + "\n\n" + s.picker.View()
	}
	return s.picker.View()
}

// pickProfileAndLogin picks the first available AWS profile and runs sso login.
//
// TODO: replace profile selection with an inline profile-picker dialog.
func pickProfileAndLogin() string {
	ctx := context.Background()

	// List profiles via aws configure list-profiles.
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "aws", "configure", "list-profiles")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("login failed: could not list profiles: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	var profiles []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			profiles = append(profiles, l)
		}
	}
	if len(profiles) == 0 {
		return "login failed: no AWS profiles found in ~/.aws/config"
	}

	// Use the single profile or fall back to the first one.
	picked := profiles[0]

	sess := awssession.New()
	sess.SetProfile(picked)

	id, err := sess.Login(ctx)
	if err != nil {
		return fmt.Sprintf("login failed: %v", err)
	}
	if id.Status != awssession.StatusOK {
		if id.Error != "" {
			return fmt.Sprintf("login failed: %s", id.Error)
		}
		return fmt.Sprintf("login failed: status %s", id.Status)
	}
	return fmt.Sprintf("logged in as %s (account %s)", picked, id.Account)
}
