package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui/components"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// ResourcesScreen shows cluster-resource browsing actions.
type ResourcesScreen struct {
	picker components.Picker
	status string
}

// NewResourcesScreen constructs the Resources action menu.
func NewResourcesScreen() *ResourcesScreen {
	items := []components.Item{
		{Label: "Namespaces", Detail: "list all namespaces"},
		{Label: "Services", Detail: "list services in namespace"},
		{Label: "Ingress", Detail: "list ingress rules"},
	}
	return &ResourcesScreen{picker: components.New("Resources", items, nil)}
}

func (s *ResourcesScreen) Init() tea.Cmd             { return nil }
func (s *ResourcesScreen) Breadcrumb() string        { return "Resources" }
func (s *ResourcesScreen) Position() (int, int)      { return s.picker.Position() }
func (s *ResourcesScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{{Key: "⏎", Action: "run"}, {Key: "?", Action: "help"}}
}

// Update routes messages to the picker and handles picker events.
func (s *ResourcesScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
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

// View renders the resources screen body.
func (s *ResourcesScreen) View(app *App) string {
	s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))
	if s.status != "" {
		return " " + theme.StatusWarn.Render(s.status) + "\n\n" + s.picker.View()
	}
	return s.picker.View()
}
