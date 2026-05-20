package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui/components"
	"github.com/Tyka95/kube-kit/internal/tui/components/picker"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// EKSConnectScreen wires `aws eks update-kubeconfig` end-to-end inside
// the TUI. Three phases driven by internal state:
//
//	1. profile — list `aws configure list-profiles`, pick one
//	2. cluster — for the chosen profile, list `aws eks list-clusters`,
//	             pick one
//	3. result  — show success/failure of `aws eks update-kubeconfig`
//
// Phase transitions happen entirely inside this screen so esc from
// any phase returns the user to the AWS submenu in one keystroke.
type EKSConnectScreen struct {
	phase   string // "profile" | "cluster" | "result"
	picker  picker.Picker
	spinner components.Spinner
	loading bool
	status  string

	// Chosen profile/region carried across phases.
	profile string
	region  string

	// Final result message rendered in the result phase.
	resultKind string // "ok" | "warn" | "error"
	resultMsg  string
}

// Internal messages.
type eksProfilesLoadedMsg struct {
	profiles []string
	err      error
}

type eksClustersLoadedMsg struct {
	clusters []string
	region   string
	err      error
}

type eksKubeconfigDoneMsg struct {
	cluster string
	err     error
	stderr  string
}

// NewEKSConnectScreen constructs the screen in the profile-loading phase.
func NewEKSConnectScreen() *EKSConnectScreen {
	return &EKSConnectScreen{
		phase:   "profile",
		picker:  picker.New("Profile", nil, nil),
		spinner: components.NewSpinner(),
		loading: true,
		status:  "Loading profiles…",
	}
}

// Breadcrumb returns the trail label.
func (s *EKSConnectScreen) Breadcrumb() string { return "EKS Connect" }

// KeyHints returns the per-phase contextual hints.
func (s *EKSConnectScreen) KeyHints() []state.KeyHint {
	switch s.phase {
	case "result":
		return []state.KeyHint{
			{Key: "esc", Action: "back"},
			{Key: "?", Action: "help"},
		}
	default:
		return []state.KeyHint{
			{Key: "⏎", Action: "select"},
			{Key: "esc", Action: "back"},
			{Key: "?", Action: "help"},
		}
	}
}

// Position satisfies PositionProvider — only meaningful in the picker phases.
func (s *EKSConnectScreen) Position() (int, int) {
	if s.phase == "result" {
		return 0, 0
	}
	return s.picker.Position()
}

// Init kicks off the profile listing.
func (s *EKSConnectScreen) Init() tea.Cmd {
	return tea.Batch(s.spinner.Start(), func() tea.Msg {
		profs, err := listAvailableProfiles()
		return eksProfilesLoadedMsg{profiles: profs, err: err}
	})
}

// Update drives phase transitions.
func (s *EKSConnectScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	switch m := msg.(type) {

	case eksProfilesLoadedMsg:
		s.loading = false
		if m.err != nil {
			s.phase = "result"
			s.resultKind = "error"
			s.resultMsg = "could not list profiles: " + m.err.Error()
			return s, nil
		}
		if len(m.profiles) == 0 {
			s.phase = "result"
			s.resultKind = "warn"
			s.resultMsg = "no profiles found in ~/.aws/config"
			return s, nil
		}
		items := make([]picker.Item, len(m.profiles))
		for i, p := range m.profiles {
			items[i] = picker.Item{Label: p, Detail: "aws profile"}
		}
		s.picker = picker.New("Profile", items, nil)
		s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))
		s.status = ""
		return s, nil

	case eksClustersLoadedMsg:
		s.loading = false
		if m.err != nil {
			s.phase = "result"
			s.resultKind = "error"
			s.resultMsg = "could not list clusters: " + m.err.Error()
			return s, nil
		}
		if len(m.clusters) == 0 {
			s.phase = "result"
			s.resultKind = "warn"
			s.resultMsg = fmt.Sprintf("no EKS clusters in %s (%s)", s.profile, m.region)
			return s, nil
		}
		s.region = m.region
		items := make([]picker.Item, len(m.clusters))
		for i, c := range m.clusters {
			items[i] = picker.Item{Label: c, Detail: m.region, Meta: s.profile}
		}
		s.picker = picker.New("Cluster", items, nil)
		s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))
		s.status = ""
		return s, nil

	case eksKubeconfigDoneMsg:
		s.loading = false
		s.phase = "result"
		if m.err != nil {
			s.resultKind = "error"
			msg := "update-kubeconfig failed: " + m.err.Error()
			if m.stderr != "" {
				msg += "\n  " + strings.TrimSpace(m.stderr)
			}
			s.resultMsg = msg
			return s, nil
		}
		s.resultKind = "ok"
		s.resultMsg = "kubectl context configured for " + m.cluster
		// Refresh app-level kube context so the header repaints.
		return s, app.loadKubeContext()

	case picker.PickerSelectedMsg:
		switch s.phase {
		case "profile":
			s.profile = m.Value
			s.phase = "cluster"
			s.loading = true
			s.status = "Loading clusters…"
			profile := s.profile
			return s, func() tea.Msg {
				clusters, region, err := listEKSClusters(profile)
				return eksClustersLoadedMsg{clusters: clusters, region: region, err: err}
			}
		case "cluster":
			cluster := m.Value
			profile := s.profile
			region := s.region
			s.loading = true
			s.status = "Configuring kubectl…"
			return s, func() tea.Msg {
				stderr, err := updateKubeconfig(profile, region, cluster)
				return eksKubeconfigDoneMsg{cluster: cluster, err: err, stderr: stderr}
			}
		}
		return s, nil

	case picker.PickerCancelMsg:
		return nil, nil

	case tea.KeyMsg:
		if s.phase == "result" {
			switch m.String() {
			case "esc", "enter", "q":
				return nil, nil
			}
		}

	case tea.WindowSizeMsg:
		s.picker.SetSize(m.Width, pickerBodyHeight(m.Height))
		return s, nil
	}

	// Spinner tick passes through.
	if cmd, handled := s.spinner.Update(msg); handled {
		return s, cmd
	}

	if s.phase != "result" {
		var cmd tea.Cmd
		s.picker, cmd = s.picker.Update(msg)
		return s, cmd
	}
	return s, nil
}

// View renders the active phase.
func (s *EKSConnectScreen) View(app *App) string {
	s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))

	if s.phase == "result" {
		body := theme.InfoCallout(s.resultKind, s.resultMsg)
		return body + "\n\n  " + theme.Dim.Render("press esc to return")
	}

	if s.loading {
		return "  " + s.spinner.View() + "  " + theme.Dim.Render(s.status)
	}

	return s.picker.View()
}

// listEKSClusters returns the list of EKS cluster names and the AWS region
// resolved for the given profile (so the caller can pass the same region
// back to update-kubeconfig — listing and updating against mismatched
// regions silently does the wrong thing).
func listEKSClusters(profile string) (clusters []string, region string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	region = resolveProfileRegion(ctx, profile)
	if region == "" {
		return nil, "", fmt.Errorf("could not resolve region for profile %s (set `region = ...` in ~/.aws/config or AWS_REGION)", profile)
	}

	var stdout, stderr bytes.Buffer
	args := []string{
		"eks", "list-clusters",
		"--profile", profile,
		"--region", region,
		"--output", "json",
	}
	cmd := exec.CommandContext(ctx, "aws", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		return nil, region, fmt.Errorf("%v: %s", runErr, strings.TrimSpace(stderr.String()))
	}

	var payload struct {
		Clusters []string `json:"clusters"`
	}
	if jsonErr := json.Unmarshal(stdout.Bytes(), &payload); jsonErr != nil {
		return nil, region, fmt.Errorf("could not parse list-clusters output: %v", jsonErr)
	}
	return payload.Clusters, region, nil
}

// resolveProfileRegion asks AWS CLI for the region configured for a
// profile. Falls back to empty string if none is set.
func resolveProfileRegion(ctx context.Context, profile string) string {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "aws", "configure", "get", "region", "--profile", profile)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}

// updateKubeconfig runs `aws eks update-kubeconfig` for the chosen
// cluster. Returns the trimmed stderr so callers can surface helpful
// failure details (auth expired, cluster not found, etc).
func updateKubeconfig(profile, region, cluster string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	args := []string{
		"eks", "update-kubeconfig",
		"--name", cluster,
		"--region", region,
		"--profile", profile,
	}
	cmd := exec.CommandContext(ctx, "aws", args...)
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stderr.String(), err
}
