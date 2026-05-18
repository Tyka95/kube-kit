package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/kctx"
	"github.com/Tyka95/kube-kit/internal/tui/components"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// contextsLoadedMsg is dispatched when the kctx.ListContexts call completes.
type contextsLoadedMsg struct {
	contexts []kctx.Context
	err      error
}

// ClusterScreen lists kubectl contexts and lets the user switch between them.
type ClusterScreen struct {
	picker  components.Picker
	items   []components.Item // mirror of picker items
	full    []string          // full context names parallel to items
	loading bool
	err     string
}

// NewClusterScreen constructs an empty ClusterScreen in the loading state.
func NewClusterScreen() *ClusterScreen {
	return &ClusterScreen{
		picker:  components.New("Cluster", nil, nil),
		loading: true,
	}
}

// Init fires the async context load.
func (s *ClusterScreen) Init() tea.Cmd {
	return func() tea.Msg {
		contexts, err := kctx.ListContexts(context.Background())
		return contextsLoadedMsg{contexts: contexts, err: err}
	}
}

// Breadcrumb returns the label pushed onto the breadcrumb trail.
func (s *ClusterScreen) Breadcrumb() string { return "Cluster" }

// KeyHints returns the per-screen contextual hints.
func (s *ClusterScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{
		{Key: "⏎", Action: "switch"},
		{Key: "?", Action: "help"},
	}
}

// Position satisfies PositionProvider so the footer can show n/total.
func (s *ClusterScreen) Position() (int, int) { return s.picker.Position() }

// Update routes messages to the picker and handles picker events.
func (s *ClusterScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case contextsLoadedMsg:
		s.loading = false
		if m.err != nil {
			s.err = m.err.Error()
			return s, nil
		}

		s.items = make([]components.Item, 0, len(m.contexts))
		s.full = make([]string, 0, len(m.contexts))

		for _, c := range m.contexts {
			var detail string
			if c.Region != "" || c.Account != "" {
				detail = fmt.Sprintf("%s  %s", c.Region, c.Account)
			} else {
				detail = "kubectl"
			}

			var meta string
			if c.Current {
				meta = "current"
			}

			s.items = append(s.items, components.Item{
				Label:  c.Cluster,
				Detail: detail,
				Meta:   meta,
			})
			s.full = append(s.full, c.Name)
		}

		s.picker = components.New("Cluster", s.items, nil)
		return s, nil

	case components.PickerSelectedMsg:
		// Find the index of the picked label in items.
		for i, it := range s.items {
			if it.Label == m.Value {
				_ = kctx.SwitchContext(context.Background(), s.full[i])
				break
			}
		}
		// Self-pop — the app will refresh state.
		return nil, nil

	case components.PickerCancelMsg:
		return nil, nil

	case tea.WindowSizeMsg:
		s.picker.SetSize(m.Width, pickerBodyHeight(m.Height))
		return s, nil
	}

	var cmd tea.Cmd
	s.picker, cmd = s.picker.Update(msg)
	return s, cmd
}

// View renders the cluster screen body.
func (s *ClusterScreen) View(app *App) string {
	if s.loading {
		return " " + theme.Dim.Render("loading contexts…")
	}
	if s.err != "" {
		return " " + theme.StatusErr.Render("error: "+s.err)
	}
	s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))
	return s.picker.View()
}
