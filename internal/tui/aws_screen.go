package tui

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

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
			// Pick a profile (fast) then exec the interactive sso login via
			// tea.ExecProcess so the device-code prompt is visible — without
			// it, bubbletea's alt-screen swallows the URL and the program
			// hangs waiting for the browser flow.
			profile, err := firstAvailableProfile()
			if err != nil {
				s.status = "login failed: " + err.Error()
				return s, nil
			}
			s.status = "running aws sso login for " + profile + "…"
			loginCmd := exec.Command("aws", "sso", "login", "--profile", profile)
			// Capture the shared App.Session so we can update the live header
			// state (not a throw-away new Session that only the AWS screen sees).
			sharedSession := app.Session
			return s, tea.ExecProcess(loginCmd, func(execErr error) tea.Msg {
				if execErr != nil {
					return awsActionDoneMsg{Status: "login failed: " + execErr.Error()}
				}
				// Update the SHARED session so app.Update's awsValidatedMsg
				// handler picks up the new identity and the header repaints.
				sharedSession.SetProfile(profile)
				id := sharedSession.Validate(context.Background(), true)
				if id.Status != awssession.StatusOK {
					return awsActionDoneMsg{Status: "login finished but validate failed: " + id.Error}
				}
				// Returning awsValidatedMsg makes the app refresh the header.
				return awsValidatedMsg{Identity: id}
			})
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
		return theme.InfoCallout("info", s.status) + "\n\n" + s.picker.View()
	}
	return s.picker.View()
}

// firstAvailableProfile returns the first profile from `aws configure list-profiles`.
//
// TODO: replace with an inline profile-picker dialog (v1.1).
func firstAvailableProfile() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "aws", "configure", "list-profiles")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("could not list profiles: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			return l, nil
		}
	}
	return "", fmt.Errorf("no profiles found in ~/.aws/config")
}
