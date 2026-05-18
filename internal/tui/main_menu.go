package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui/components"
	"github.com/Tyka95/kube-kit/internal/tui/state"
)

// MainMenu is the root screen.
type MainMenu struct {
	picker components.Picker
}

// NewMainMenu constructs the main menu.
func NewMainMenu() *MainMenu {
	items := []components.Item{
		{Label: "Pods", Detail: "list · logs · shell · inspect"},
		{Label: "Deployments", Detail: "browse · scale · restart"},
		{Label: "Resources", Detail: "namespaces · services · ingress"},
		{Label: "Cluster", Detail: "context · nodes"},
		{Label: "Database", Detail: "tunnel via socat pod"},
		{Label: "AWS", Detail: "sso · eks · s3"},
		{Label: "Exit", Detail: ""},
	}
	return &MainMenu{picker: components.New("Main Menu", items, nil)}
}

func (m *MainMenu) Init() tea.Cmd                  { return nil }
func (m *MainMenu) Breadcrumb() string             { return "" }
func (m *MainMenu) KeyHints() []state.KeyHint      { return []state.KeyHint{{Key: "?", Action: "help"}} }
func (m *MainMenu) Position() (int, int)           { return m.picker.Position() }

// Update routes events to the picker and handles its emitted messages.
func (m *MainMenu) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.picker.SetSize(ws.Width, pickerBodyHeight(ws.Height))
	}

	switch v := msg.(type) {
	case components.PickerSelectedMsg:
		switch v.Value {
		case "Exit":
			return m, func() tea.Msg { return QuitMsg{} }
		case "Pods":
			app.Push(NewPodsScreen())
		case "Deployments":
			app.Push(NewDeploymentsScreen())
		case "Resources":
			app.Push(NewResourcesScreen())
		case "Cluster":
			app.Push(NewClusterScreen())
		case "Database":
			app.Push(NewDatabaseScreen(app.Config, app.Session, app.Discover))
		case "AWS":
			app.Push(NewAWSScreen())
		}
		return m, nil
	case components.PickerCancelMsg:
		// Esc/q at root menu is a no-op (k9s convention).
		return m, nil
	case components.PickerHelpMsg:
		app.Push(NewHelpScreen(m.KeyHints()))
		return m, nil
	}

	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	return m, cmd
}

// View renders the picker body.
func (m *MainMenu) View(app *App) string {
	m.picker.SetSize(app.Width, pickerBodyHeight(app.Height))
	return m.picker.View()
}

func pickerBodyHeight(h int) int {
	body := h - components.HeaderHeight - components.FooterHeight - 2
	if body < 3 {
		body = 3
	}
	return body
}
