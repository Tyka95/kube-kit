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

// S3BucketsScreen lists S3 buckets visible to a chosen AWS profile.
// Two phases: pick profile → list buckets. The list is browsable but
// non-interactive — selection just shows the bucket name for copy;
// deeper drill-down (objects, prefixes) is out of scope for v0.1.
type S3BucketsScreen struct {
	phase   string // "profile" | "buckets" | "result"
	picker  picker.Picker
	spinner components.Spinner
	loading bool
	status  string

	profile    string
	resultKind string
	resultMsg  string
}

type s3BucketsLoadedMsg struct {
	buckets []s3BucketInfo
	err     error
}

type s3BucketInfo struct {
	Name         string
	CreationDate string
}

// NewS3BucketsScreen constructs the screen in the profile-loading phase.
func NewS3BucketsScreen() *S3BucketsScreen {
	return &S3BucketsScreen{
		phase:   "profile",
		picker:  picker.New("Profile", nil, nil),
		spinner: components.NewSpinner(),
		loading: true,
		status:  "Loading profiles…",
	}
}

// Breadcrumb returns the trail label.
func (s *S3BucketsScreen) Breadcrumb() string { return "S3 Buckets" }

// KeyHints returns per-phase contextual hints.
func (s *S3BucketsScreen) KeyHints() []state.KeyHint {
	if s.phase == "result" {
		return []state.KeyHint{
			{Key: "esc", Action: "back"},
			{Key: "?", Action: "help"},
		}
	}
	return []state.KeyHint{
		{Key: "⏎", Action: "select"},
		{Key: "esc", Action: "back"},
		{Key: "?", Action: "help"},
	}
}

// Position satisfies PositionProvider.
func (s *S3BucketsScreen) Position() (int, int) {
	if s.phase == "result" {
		return 0, 0
	}
	return s.picker.Position()
}

// Init starts the profile-list load.
func (s *S3BucketsScreen) Init() tea.Cmd {
	return tea.Batch(s.spinner.Start(), func() tea.Msg {
		profs, err := listAvailableProfiles()
		return eksProfilesLoadedMsg{profiles: profs, err: err}
	})
}

// Update drives the two-phase flow.
func (s *S3BucketsScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
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

	case s3BucketsLoadedMsg:
		s.loading = false
		if m.err != nil {
			s.phase = "result"
			s.resultKind = "error"
			s.resultMsg = "could not list buckets: " + m.err.Error()
			return s, nil
		}
		if len(m.buckets) == 0 {
			s.phase = "result"
			s.resultKind = "warn"
			s.resultMsg = fmt.Sprintf("no buckets visible to %s", s.profile)
			return s, nil
		}
		items := make([]picker.Item, len(m.buckets))
		for i, b := range m.buckets {
			items[i] = picker.Item{Label: b.Name, Detail: b.CreationDate, Meta: s.profile}
		}
		s.picker = picker.New("Buckets", items, nil)
		s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))
		s.status = ""
		return s, nil

	case picker.PickerSelectedMsg:
		switch s.phase {
		case "profile":
			s.profile = m.Value
			s.phase = "buckets"
			s.loading = true
			s.status = "Loading buckets…"
			profile := s.profile
			return s, func() tea.Msg {
				buckets, err := listS3Buckets(profile)
				return s3BucketsLoadedMsg{buckets: buckets, err: err}
			}
		case "buckets":
			// Surface the bucket name in the status line so the user can
			// copy it; no drill-down yet.
			s.status = "selected: " + m.Value
			return s, nil
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
func (s *S3BucketsScreen) View(app *App) string {
	s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))

	if s.phase == "result" {
		return theme.InfoCallout(s.resultKind, s.resultMsg) +
			"\n\n  " + theme.Dim.Render("press esc to return")
	}

	if s.loading {
		return "  " + s.spinner.View() + "  " + theme.Dim.Render(s.status)
	}

	if s.status != "" {
		return theme.InfoCallout("info", s.status) + "\n\n" + s.picker.View()
	}
	return s.picker.View()
}

// listS3Buckets returns every bucket the profile can list via
// `aws s3api list-buckets`. Buckets are global, so no region needed.
func listS3Buckets(profile string) ([]s3BucketInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	args := []string{
		"s3api", "list-buckets",
		"--profile", profile,
		"--output", "json",
	}
	cmd := exec.CommandContext(ctx, "aws", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}

	var payload struct {
		Buckets []struct {
			Name         string `json:"Name"`
			CreationDate string `json:"CreationDate"`
		} `json:"Buckets"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		return nil, fmt.Errorf("could not parse list-buckets output: %v", err)
	}

	out := make([]s3BucketInfo, len(payload.Buckets))
	for i, b := range payload.Buckets {
		out[i] = s3BucketInfo{Name: b.Name, CreationDate: b.CreationDate}
	}
	return out, nil
}
