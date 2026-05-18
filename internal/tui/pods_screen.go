package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui/components"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// PodsScreen shows pod-related actions.
type PodsScreen struct {
	picker components.Picker
	status string
}

// NewPodsScreen constructs the Pods action menu.
func NewPodsScreen() *PodsScreen {
	items := []components.Item{
		{Label: "List Pods", Detail: "show all pods in namespace"},
		{Label: "View Logs", Detail: "tail pod logs"},
		{Label: "Open Shell", Detail: "exec -it /bin/sh"},
		{Label: "Inspect", Detail: "describe pod"},
	}
	return &PodsScreen{picker: components.New("Pods", items, nil)}
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
	case components.PickerSelectedMsg:
		switch v.Value {
		case "List Pods":
			app.Push(NewPodListScreen(app.KubeNamespace))
			return s, nil
		default:
			s.status = v.Value + ": not yet implemented"
			return s, nil
		}
	case components.PickerCancelMsg:
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
