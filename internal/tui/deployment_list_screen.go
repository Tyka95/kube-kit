package tui

import (
	"context"
	"os/exec"
	"path"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui/components"
	"github.com/Tyka95/kube-kit/internal/tui/components/picker"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// deploymentRow holds the data parsed from a single kubectl output line.
type deploymentRow struct {
	name    string
	ready   string
	desired string
	image   string
}

// deploymentActionLoadedMsg is dispatched when the kubectl call completes.
type deploymentActionLoadedMsg struct {
	rows []deploymentRow
	err  error
}

// deploymentActionStatusMsg is dispatched after an action completes.
type deploymentActionStatusMsg struct {
	kind string // "info" | "ok" | "warn" | "error"
	text string
}

// DeploymentAction describes what happens once a deployment is picked.
type DeploymentAction struct {
	Name       string // breadcrumb: "browse" / "scale" / "restart"
	OnSelected func(dep string) tea.Cmd
}

// DeploymentListScreen lists deployments in the current namespace.
type DeploymentListScreen struct {
	namespace string
	action    DeploymentAction
	picker    picker.Picker
	spinner   components.Spinner
	loading   bool
	err       error
	status    string
}

// NewDeploymentListScreen constructs a DeploymentListScreen in the loading state.
func NewDeploymentListScreen(namespace string, action DeploymentAction) *DeploymentListScreen {
	binds := []picker.Bind{
		{Key: "r", Action: "refresh"},
	}
	return &DeploymentListScreen{
		namespace: namespace,
		action:    action,
		picker:    picker.New(action.Name, nil, binds),
		spinner:   components.NewSpinner(),
		loading:   true,
	}
}

// Init starts the deployment load and spinner.
func (s *DeploymentListScreen) Init() tea.Cmd {
	return tea.Batch(s.loadDeployments(), s.spinner.Start())
}

// loadDeployments returns a Cmd that shells out to kubectl and dispatches deploymentActionLoadedMsg.
func (s *DeploymentListScreen) loadDeployments() tea.Cmd {
	ns := s.namespace
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		args := []string{
			"get", "deployments",
			"--no-headers",
			"-o", "custom-columns=:metadata.name,:status.readyReplicas,:spec.replicas,:spec.template.spec.containers[0].image",
		}
		if ns != "" {
			args = append(args, "-n", ns)
		}
		cmd := exec.CommandContext(ctx, "kubectl", args...)
		out, err := cmd.Output()
		if err != nil {
			return deploymentActionLoadedMsg{err: err}
		}

		var rows []deploymentRow
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			row := deploymentRow{
				name:    fields[0],
				ready:   "0",
				desired: "0",
				image:   "—",
			}
			if len(fields) >= 2 && fields[1] != "<none>" {
				row.ready = fields[1]
			}
			if len(fields) >= 3 && fields[2] != "<none>" {
				row.desired = fields[2]
			}
			if len(fields) >= 4 && fields[3] != "<none>" {
				// Strip tag: everything after the last ':', then take the last path segment.
				img := fields[3]
				if colonIdx := strings.LastIndex(img, ":"); colonIdx != -1 {
					img = img[:colonIdx]
				}
				row.image = path.Base(img)
			}
			rows = append(rows, row)
		}
		return deploymentActionLoadedMsg{rows: rows}
	}
}

// Breadcrumb returns the action name for display in the nav bar.
func (s *DeploymentListScreen) Breadcrumb() string { return s.action.Name }

// KeyHints returns the key hints for the status bar.
func (s *DeploymentListScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{
		{Key: "⏎", Action: s.action.Name},
		{Key: "r", Action: "refresh"},
		{Key: "?", Action: "help"},
	}
}

// Position returns the picker cursor position.
func (s *DeploymentListScreen) Position() (int, int) { return s.picker.Position() }

// Update handles all incoming messages.
func (s *DeploymentListScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		s.picker.SetSize(m.Width, pickerBodyHeight(m.Height))
		return s, nil

	case deploymentActionLoadedMsg:
		s.loading = false
		s.spinner.Stop()
		s.err = m.err
		if m.err != nil {
			return s, nil
		}
		items := make([]picker.Item, 0, len(m.rows))
		for _, row := range m.rows {
			items = append(items, picker.Item{
				Label:  row.name,
				Detail: row.ready + "/" + row.desired,
				Meta:   "image:" + row.image,
			})
		}
		binds := []picker.Bind{{Key: "r", Action: "refresh"}}
		s.picker = picker.New(s.action.Name, items, binds)
		return s, nil

	case picker.PickerSelectedMsg:
		if s.action.OnSelected != nil {
			return s, s.action.OnSelected(m.Value)
		}
		return s, nil

	case picker.PickerActionMsg:
		if m.Action == "refresh" {
			s.loading = true
			s.err = nil
			s.status = ""
			return s, tea.Batch(s.loadDeployments(), s.spinner.Start())
		}
		return s, nil

	case picker.PickerCancelMsg:
		s.spinner.Stop()
		return nil, nil

	case deploymentActionStatusMsg:
		s.status = theme.InfoCallout(m.kind, m.text)
		return s, nil
	}

	// Spinner tick — keeps the loading animation alive.
	if cmd, handled := s.spinner.Update(msg); handled {
		return s, cmd
	}

	var cmd tea.Cmd
	s.picker, cmd = s.picker.Update(msg)
	return s, cmd
}

// View renders the deployment list screen body.
func (s *DeploymentListScreen) View(app *App) string {
	if s.loading {
		return "  " + s.spinner.View() + "  " + theme.Dim.Render("loading deployments…")
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
