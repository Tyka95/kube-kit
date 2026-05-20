package tui

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui/components/picker"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// SSOProfilePickerScreen lists AWS profiles from `aws configure list-profiles`
// and lets the user pick the one to SSO-login with. Before this screen
// existed the AWS submenu silently auto-picked the first profile, which
// always sent users into `stage` even when they wanted `prod`.
type SSOProfilePickerScreen struct {
	picker  picker.Picker
	loading bool
	status  string
}

// profilesLoadedMsg carries the result of `aws configure list-profiles`.
type profilesLoadedMsg struct {
	profiles []string
	err      error
}

// NewSSOProfilePickerScreen constructs the screen in the loading state.
func NewSSOProfilePickerScreen() *SSOProfilePickerScreen {
	return &SSOProfilePickerScreen{
		picker:  picker.New("SSO Profile", nil, nil),
		loading: true,
		status:  "Loading profiles…",
	}
}

// Breadcrumb returns the trail label.
func (s *SSOProfilePickerScreen) Breadcrumb() string { return "Profile" }

// KeyHints returns contextual key hints.
func (s *SSOProfilePickerScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{
		{Key: "⏎", Action: "login"},
		{Key: "esc", Action: "back"},
		{Key: "?", Action: "help"},
	}
}

// Position satisfies PositionProvider so the footer can show n/total.
func (s *SSOProfilePickerScreen) Position() (int, int) { return s.picker.Position() }

// Init fires the profile-listing command.
func (s *SSOProfilePickerScreen) Init() tea.Cmd {
	return func() tea.Msg {
		profs, err := listAvailableProfiles()
		return profilesLoadedMsg{profiles: profs, err: err}
	}
}

// Update routes messages to the picker and handles profile-load events.
func (s *SSOProfilePickerScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	switch m := msg.(type) {

	case profilesLoadedMsg:
		s.loading = false
		if m.err != nil {
			s.status = "Error: " + m.err.Error()
			return s, nil
		}
		if len(m.profiles) == 0 {
			s.status = "No profiles found in ~/.aws/config"
			return s, nil
		}
		items := make([]picker.Item, len(m.profiles))
		for i, p := range m.profiles {
			items[i] = picker.Item{Label: p, Detail: "aws sso login --profile " + p}
		}
		s.picker = picker.New("SSO Profile", items, nil)
		s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))
		s.status = ""
		return s, nil

	case picker.PickerSelectedMsg:
		// Pop ourselves off and push the SSO login screen with the chosen
		// profile. Pop-then-push keeps the back-trail clean: esc from the
		// SSO screen returns to the AWS submenu, not back to this picker.
		app.Pop()
		app.Push(NewSSOLoginScreen(m.Value, app.Session))
		return app.Current(), nil

	case picker.PickerCancelMsg:
		return nil, nil

	case tea.WindowSizeMsg:
		s.picker.SetSize(m.Width, pickerBodyHeight(m.Height))
		return s, nil
	}

	var cmd tea.Cmd
	s.picker, cmd = s.picker.Update(msg)
	return s, cmd
}

// View renders the profile picker.
func (s *SSOProfilePickerScreen) View(app *App) string {
	s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))
	if s.status != "" {
		return theme.Dim.Render(s.status)
	}
	return s.picker.View()
}

// listAvailableProfiles returns every profile name from
// `aws configure list-profiles` in declaration order, trimmed and deduped.
func listAvailableProfiles() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "aws", "configure", "list-profiles")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("could not list profiles: %v", err)
	}

	seen := make(map[string]bool)
	var out []string
	for _, raw := range strings.Split(stdout.String(), "\n") {
		p := strings.TrimSpace(raw)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out, nil
}
