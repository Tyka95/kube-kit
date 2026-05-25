package tui

import (
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui/components/picker"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// PodsScreen shows pod-related actions.
type PodsScreen struct {
	picker picker.Picker
	status string
}

// NewPodsScreen constructs the Pods action menu.
func NewPodsScreen() *PodsScreen {
	items := []picker.Item{
		{Label: "List Pods", Detail: "show all pods in namespace"},
		{Label: "View Logs", Detail: "tail pod logs"},
		{Label: "Open Shell", Detail: "exec -it /bin/sh"},
		{Label: "Inspect", Detail: "describe pod"},
	}
	return &PodsScreen{picker: picker.New("Pods", items, nil)}
}

func (s *PodsScreen) Init() tea.Cmd             { return nil }
func (s *PodsScreen) Breadcrumb() string        { return "Pods" }
func (s *PodsScreen) Position() (int, int)      { return s.picker.Position() }
func (s *PodsScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{{Key: "⏎", Action: "run"}, {Key: "?", Action: "help"}}
}

// Update routes messages to the picker and handles picker events.
func (s *PodsScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		s.picker.SetSize(ws.Width, pickerBodyHeight(ws.Height))
		return s, nil
	}

	switch v := msg.(type) {
	case picker.PickerSelectedMsg:
		switch v.Value {
		case "List Pods":
			app.Push(NewPodListScreen(app.KubeNamespace))
			return s, nil
		case "View Logs":
			ns := app.KubeNamespace
			app.Push(NewPodActionScreen(ns, PodAction{
				Name: "logs",
				OnSelected: func(pod string) tea.Cmd {
					cmd := exec.Command("kubectl", "logs", "-f", "--tail=200", pod, "-n", ns)
					return tea.ExecProcess(cmd, func(err error) tea.Msg {
						if err != nil {
							return podActionStatusMsg{kind: "error", text: "kubectl logs: " + err.Error()}
						}
						return podActionStatusMsg{kind: "info", text: "logs closed for " + pod}
					})
				},
			}))
			return s, nil
		case "Open Shell":
			ns := app.KubeNamespace
			app.Push(NewPodActionScreen(ns, PodAction{
				Name: "shell",
				OnSelected: func(pod string) tea.Cmd {
					cmd := exec.Command("kubectl", "exec", "-it", pod, "-n", ns, "--", "/bin/sh")
					return tea.ExecProcess(cmd, func(err error) tea.Msg {
						if err != nil {
							return podActionStatusMsg{kind: "error", text: "kubectl exec: " + err.Error()}
						}
						return podActionStatusMsg{kind: "info", text: "shell closed for " + pod}
					})
				},
			}))
			return s, nil
		case "Inspect":
			ns := app.KubeNamespace
			// Inline renderer instead of tea.ExecProcess+less. Old flow
			// suspended bubbletea entirely so the user lost the header,
			// breadcrumb, footer and key hints — only way out was less's
			// `q` convention. New screen keeps the chrome and uses the
			// viewport bubble for scrolling, with esc/q both exiting.
			app.Push(NewPodActionScreen(ns, PodAction{
				Name: "inspect",
				OnSelected: func(pod string) tea.Cmd {
					return func() tea.Msg {
						return pushInspectMsg{pod: pod, namespace: ns}
					}
				},
			}))
			return s, nil
		default:
			s.status = v.Value + ": not yet implemented"
		}
		return s, nil
	case picker.PickerCancelMsg:
		return nil, nil
	}

	var cmd tea.Cmd
	s.picker, cmd = s.picker.Update(msg)
	return s, cmd
}

// View renders the pods screen body.
func (s *PodsScreen) View(app *App) string {
	s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))
	if s.status != "" {
		return theme.InfoCallout("warn", s.status) + "\n\n" + s.picker.View()
	}
	return s.picker.View()
}
