package tui

import (
	"context"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui/components"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// PodAction describes what happens once a pod is picked.
type PodAction struct {
	Name       string // breadcrumb name: "logs" / "shell" / "inspect"
	OnSelected func(pod string) tea.Cmd
}

type podActionStatusMsg struct {
	kind string // "info" | "warn" | "error"
	text string
}

type podActionLoadedMsg struct {
	rows []podRow
	err  error
}

// PodActionScreen is a generic "pick a pod, run an action" screen.
type PodActionScreen struct {
	namespace string
	action    PodAction
	picker    components.Picker
	spinner   components.Spinner
	loading   bool
	err       error
	status    string
}

// NewPodActionScreen constructs a PodActionScreen for the given namespace and action.
func NewPodActionScreen(namespace string, action PodAction) *PodActionScreen {
	binds := []components.Bind{{Key: "r", Action: "refresh"}}
	return &PodActionScreen{
		namespace: namespace,
		action:    action,
		picker:    components.New(action.Name, nil, binds),
		spinner:   components.NewSpinner(),
		loading:   true,
	}
}

// Init starts the pod load and spinner.
func (s *PodActionScreen) Init() tea.Cmd {
	return tea.Batch(s.loadPods(), s.spinner.Start())
}

func (s *PodActionScreen) loadPods() tea.Cmd {
	ns := s.namespace
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		args := []string{"get", "pods", "--no-headers",
			"-o", "custom-columns=:metadata.name,:status.phase,:status.containerStatuses[0].restartCount"}
		if ns != "" {
			args = append(args, "-n", ns)
		}
		cmd := exec.CommandContext(ctx, "kubectl", args...)
		out, err := cmd.Output()
		if err != nil {
			return podActionLoadedMsg{err: err}
		}
		var rows []podRow
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			row := podRow{name: fields[0], phase: "Unknown", restarts: "0"}
			if len(fields) >= 2 {
				row.phase = fields[1]
			}
			if len(fields) >= 3 && fields[2] != "<none>" {
				row.restarts = fields[2]
			}
			rows = append(rows, row)
		}
		return podActionLoadedMsg{rows: rows}
	}
}

// Breadcrumb returns the action name for display in the nav bar.
func (s *PodActionScreen) Breadcrumb() string { return s.action.Name }

// KeyHints returns the key hints for the status bar.
func (s *PodActionScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{{Key: "⏎", Action: s.action.Name}, {Key: "r", Action: "refresh"}, {Key: "?", Action: "help"}}
}

// Position returns the picker cursor position.
func (s *PodActionScreen) Position() (int, int) { return s.picker.Position() }

// Update handles all incoming messages.
func (s *PodActionScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		s.picker.SetSize(m.Width, pickerBodyHeight(m.Height))
		return s, nil
	case podActionLoadedMsg:
		s.loading = false
		s.spinner.Stop()
		s.err = m.err
		if m.err != nil {
			return s, nil
		}
		items := make([]components.Item, 0, len(m.rows))
		for _, row := range m.rows {
			items = append(items, components.Item{Label: row.name, Detail: row.phase, Meta: "restarts: " + row.restarts})
		}
		binds := []components.Bind{{Key: "r", Action: "refresh"}}
		s.picker = components.New(s.action.Name, items, binds)
		return s, nil
	case components.PickerSelectedMsg:
		if s.action.OnSelected != nil {
			return s, s.action.OnSelected(m.Value)
		}
		return s, nil
	case components.PickerActionMsg:
		if m.Action == "refresh" {
			s.loading = true
			s.err = nil
			s.status = ""
			return s, tea.Batch(s.loadPods(), s.spinner.Start())
		}
		return s, nil
	case components.PickerCancelMsg:
		s.spinner.Stop()
		return nil, nil
	case podActionStatusMsg:
		s.status = theme.InfoCallout(m.kind, m.text)
		return s, nil
	}
	if cmd, handled := s.spinner.Update(msg); handled {
		return s, cmd
	}
	var cmd tea.Cmd
	s.picker, cmd = s.picker.Update(msg)
	return s, cmd
}

// View renders the screen.
func (s *PodActionScreen) View(app *App) string {
	if s.loading {
		return "  " + s.spinner.View() + "  " + theme.Dim.Render("loading pods…")
	}
	if s.err != nil {
		return theme.InfoCallout("error", "kubectl error: "+s.err.Error())
	}
	s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))
	if s.status != "" {
		return s.status + "\n\n" + s.picker.View()
	}
	return s.picker.View()
}
