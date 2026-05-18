package tui

import (
	"context"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui/components"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// DeploymentsScreen shows deployment-related actions.
type DeploymentsScreen struct {
	picker components.Picker
	status string
}

// NewDeploymentsScreen constructs the Deployments action menu.
func NewDeploymentsScreen() *DeploymentsScreen {
	items := []components.Item{
		{Label: "Browse", Detail: "list deployments in namespace"},
		{Label: "Scale", Detail: "set replica count"},
		{Label: "Restart", Detail: "rollout restart"},
	}
	return &DeploymentsScreen{picker: components.New("Deployments", items, nil)}
}

func (s *DeploymentsScreen) Init() tea.Cmd             { return nil }
func (s *DeploymentsScreen) Breadcrumb() string        { return "Deployments" }
func (s *DeploymentsScreen) Position() (int, int)      { return s.picker.Position() }
func (s *DeploymentsScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{{Key: "⏎", Action: "run"}, {Key: "?", Action: "help"}}
}

// Update routes messages to the picker and handles picker events.
func (s *DeploymentsScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		s.picker.SetSize(ws.Width, pickerBodyHeight(ws.Height))
		return s, nil
	}

	switch v := msg.(type) {
	case components.PickerSelectedMsg:
		switch v.Value {
		case "Browse":
			ns := app.KubeNamespace
			app.Push(NewDeploymentListScreen(ns, DeploymentAction{
				Name: "browse",
				OnSelected: func(dep string) tea.Cmd {
					cmd := exec.Command("sh", "-c",
						`f=$(mktemp) && kubectl describe deployment `+shEscape(dep)+` -n `+shEscape(ns)+` > "$f" 2>&1 && less -R "$f"; rm -f "$f"`)
					return tea.ExecProcess(cmd, func(err error) tea.Msg {
						if err != nil {
							return deploymentActionStatusMsg{kind: "error", text: "kubectl describe: " + err.Error()}
						}
						return deploymentActionStatusMsg{kind: "info", text: "closed: " + dep}
					})
				},
			}))
			return s, nil

		case "Scale":
			ns := app.KubeNamespace
			app.Push(NewDeploymentListScreen(ns, DeploymentAction{
				Name: "scale",
				OnSelected: func(dep string) tea.Cmd {
					// gum input runs in a real terminal — tea.ExecProcess suspends bubbletea
					// and pipes the entered replicas to kubectl scale.
					cmd := exec.Command("sh", "-c",
						"replicas=$(gum input --placeholder=\"new replica count\" --prompt=\"replicas › \") && "+
							"[ -n \"$replicas\" ] && kubectl scale deployment "+dep+" --replicas=\"$replicas\" -n "+ns)
					return tea.ExecProcess(cmd, func(err error) tea.Msg {
						if err != nil {
							return deploymentActionStatusMsg{kind: "error", text: "scale failed: " + err.Error()}
						}
						return deploymentActionStatusMsg{kind: "ok", text: "scaled: " + dep}
					})
				},
			}))
			return s, nil

		case "Restart":
			ns := app.KubeNamespace
			app.Push(NewDeploymentListScreen(ns, DeploymentAction{
				Name: "restart",
				OnSelected: func(dep string) tea.Cmd {
					return func() tea.Msg {
						ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
						defer cancel()
						err := exec.CommandContext(ctx, "kubectl", "rollout", "restart", "deployment/"+dep, "-n", ns).Run()
						if err != nil {
							return deploymentActionStatusMsg{kind: "error", text: "rollout restart: " + err.Error()}
						}
						return deploymentActionStatusMsg{kind: "ok", text: "restarted: " + dep}
					}
				},
			}))
			return s, nil

		default:
			s.status = v.Value + ": not yet implemented"
		}
		return s, nil

	case components.PickerCancelMsg:
		return nil, nil
	}

	var cmd tea.Cmd
	s.picker, cmd = s.picker.Update(msg)
	return s, cmd
}

// View renders the deployments screen body.
func (s *DeploymentsScreen) View(app *App) string {
	s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))
	if s.status != "" {
		return theme.InfoCallout("warn", s.status) + "\n\n" + s.picker.View()
	}
	return s.picker.View()
}
