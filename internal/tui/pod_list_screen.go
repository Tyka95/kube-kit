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

// podRow holds the data parsed from a single kubectl output line.
type podRow struct {
	name     string
	phase    string
	restarts string
}

// podsLoadedMsg is dispatched when the kubectl call completes.
type podsLoadedMsg struct {
	rows []podRow
	err  error
}

// PodListScreen lists pods in the current namespace.
type PodListScreen struct {
	namespace string
	picker    components.Picker
	loading   bool
	err       error
	status    string
}

// NewPodListScreen constructs a PodListScreen in the loading state.
func NewPodListScreen(namespace string) *PodListScreen {
	binds := []components.Bind{
		{Key: "r", Action: "refresh"},
	}
	return &PodListScreen{
		namespace: namespace,
		picker:    components.New("Pod List", nil, binds),
		loading:   true,
	}
}

// Init fires the async kubectl call.
func (s *PodListScreen) Init() tea.Cmd {
	return s.loadPods()
}

// loadPods returns a Cmd that shells out to kubectl and dispatches podsLoadedMsg.
func (s *PodListScreen) loadPods() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
			"--no-headers",
			"-o", "custom-columns=:metadata.name,:status.phase,:status.containerStatuses[0].restartCount")

		out, err := cmd.Output()
		if err != nil {
			return podsLoadedMsg{err: err}
		}

		var rows []podRow
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			row := podRow{
				name:     fields[0],
				phase:    "Unknown",
				restarts: "0",
			}
			if len(fields) >= 2 {
				row.phase = fields[1]
			}
			if len(fields) >= 3 && fields[2] != "<none>" {
				row.restarts = fields[2]
			}
			rows = append(rows, row)
		}
		return podsLoadedMsg{rows: rows}
	}
}

// Breadcrumb returns the label pushed onto the breadcrumb trail.
func (s *PodListScreen) Breadcrumb() string { return "Pods · list" }

// KeyHints returns the per-screen contextual hints.
func (s *PodListScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{
		{Key: "⏎", Action: "details"},
		{Key: "r", Action: "refresh"},
		{Key: "?", Action: "help"},
	}
}

// Position satisfies PositionProvider so the footer can show n/total.
func (s *PodListScreen) Position() (int, int) { return s.picker.Position() }

// Update routes messages to the picker and handles picker events.
func (s *PodListScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		s.picker.SetSize(m.Width, pickerBodyHeight(m.Height))
		return s, nil

	case podsLoadedMsg:
		s.loading = false
		s.err = m.err
		if m.err != nil {
			return s, nil
		}
		items := make([]components.Item, 0, len(m.rows))
		for _, row := range m.rows {
			items = append(items, components.Item{
				Label:  row.name,
				Detail: row.phase,
				Meta:   "restarts: " + row.restarts,
			})
		}
		binds := []components.Bind{
			{Key: "r", Action: "refresh"},
		}
		s.picker = components.New("Pod List", items, binds)
		return s, nil

	case components.PickerSelectedMsg:
		s.status = theme.InfoCallout("info", "selected: "+m.Value+" (drill-in not yet implemented)")
		return s, nil

	case components.PickerActionMsg:
		if m.Action == "refresh" {
			s.loading = true
			s.err = nil
			s.status = ""
			return s, s.loadPods()
		}
		return s, nil

	case components.PickerCancelMsg:
		return nil, nil
	}

	var cmd tea.Cmd
	s.picker, cmd = s.picker.Update(msg)
	return s, cmd
}

// View renders the pod list screen body.
func (s *PodListScreen) View(app *App) string {
	if s.loading {
		return theme.Dim.Render("loading pods…")
	}
	if s.err != nil {
		return theme.StatusErr.Render("kubectl error: " + s.err.Error())
	}
	s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))
	if s.status != "" {
		return s.status + "\n\n" + s.picker.View()
	}
	return s.picker.View()
}
