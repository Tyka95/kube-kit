package tui

import (
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
		s.status = v.Value + ": not yet implemented"
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
