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

// ResourceKind identifies which Kubernetes resource type to list.
type ResourceKind string

const (
	KindNamespaces ResourceKind = "namespaces"
	KindServices   ResourceKind = "services"
	KindIngress    ResourceKind = "ingress"
)

// ResourceAction describes how to fetch and act on a resource list.
type ResourceAction struct {
	Kind      ResourceKind
	Namespace string // empty for cluster-scoped resources (e.g. namespaces)
	Name      string // breadcrumb label

	// OnSelected is called when the user presses Enter on a row. When nil the
	// default handler opens 'kubectl describe <kind> <name>' in less.
	OnSelected func(name string) tea.Cmd
}

type resourceRow struct {
	label  string
	detail string
	meta   string
}

type resourceListLoadedMsg struct {
	rows []resourceRow
	err  error
}

type resourceActionStatusMsg struct {
	kind string // "info" | "warn" | "error" | "ok"
	text string
}

// ResourceListScreen is a generic list-with-action screen parameterised by
// resource type.
type ResourceListScreen struct {
	action  ResourceAction
	picker  components.Picker
	spinner components.Spinner
	loading bool
	err     error
	status  string
}

// NewResourceListScreen constructs a ResourceListScreen in the loading state.
func NewResourceListScreen(action ResourceAction) *ResourceListScreen {
	binds := []components.Bind{{Key: "r", Action: "refresh"}}
	name := action.Name
	if name == "" {
		name = string(action.Kind)
	}
	return &ResourceListScreen{
		action:  action,
		picker:  components.New(name, nil, binds),
		spinner: components.NewSpinner(),
		loading: true,
	}
}

// Init fires the async kubectl call and starts the loading spinner.
func (s *ResourceListScreen) Init() tea.Cmd {
	return tea.Batch(s.loadResources(), s.spinner.Start())
}

// loadResources builds and runs the appropriate kubectl command.
func (s *ResourceListScreen) loadResources() tea.Cmd {
	kind := s.action.Kind
	ns := s.action.Namespace
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		var args []string
		switch kind {
		case KindNamespaces:
			args = []string{
				"get", "namespaces",
				"--no-headers",
				"-o", "custom-columns=:metadata.name,:status.phase",
			}
		case KindServices:
			args = []string{
				"get", "services",
				"--no-headers",
				"-o", "custom-columns=:metadata.name,:spec.type,:spec.clusterIP,:spec.ports[*].port",
			}
			if ns != "" {
				args = append(args, "-n", ns)
			}
		case KindIngress:
			args = []string{
				"get", "ingress",
				"--no-headers",
				"-o", "custom-columns=:metadata.name,:spec.rules[*].host,:status.loadBalancer.ingress[0].ip",
			}
			if ns != "" {
				args = append(args, "-n", ns)
			}
		}

		cmd := exec.CommandContext(ctx, "kubectl", args...)
		out, err := cmd.Output()
		if err != nil {
			return resourceListLoadedMsg{err: err}
		}

		var rows []resourceRow
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			row := parseRow(kind, fields)
			rows = append(rows, row)
		}
		return resourceListLoadedMsg{rows: rows}
	}
}

// parseRow converts raw kubectl output fields into a resourceRow based on kind.
func parseRow(kind ResourceKind, fields []string) resourceRow {
	none := func(s string) string {
		if s == "<none>" || s == "" {
			return "—"
		}
		return s
	}

	switch kind {
	case KindNamespaces:
		row := resourceRow{label: fields[0]}
		if len(fields) >= 2 {
			row.detail = none(fields[1])
		}
		return row

	case KindServices:
		row := resourceRow{label: fields[0]}
		if len(fields) >= 2 {
			row.detail = none(fields[1]) // type
		}
		if len(fields) >= 3 {
			row.meta = none(fields[2]) // clusterIP
		}
		if len(fields) >= 4 {
			ports := none(fields[3])
			if ports != "—" {
				row.meta = row.meta + "  ports:" + ports
			}
		}
		return row

	case KindIngress:
		row := resourceRow{label: fields[0]}
		if len(fields) >= 2 {
			row.detail = none(fields[1]) // hosts
		}
		if len(fields) >= 3 {
			row.meta = none(fields[2]) // lb IP
		}
		return row

	default:
		if len(fields) > 0 {
			return resourceRow{label: fields[0]}
		}
		return resourceRow{}
	}
}

// defaultDescribe returns a Cmd that opens 'kubectl describe <kind> <name>' in
// less via tea.ExecProcess. The output is buffered to a temp file first so
// less's stdin remains the terminal — without that, piping kubectl | less
// leaves less unable to receive the 'q' keystroke to quit.
func defaultDescribe(kind ResourceKind, ns, name string) tea.Cmd {
	args := "describe " + shEscape(string(kind)) + " " + shEscape(name)
	if ns != "" {
		args += " -n " + shEscape(ns)
	}
	shCmd := `f=$(mktemp) && kubectl ` + args + ` > "$f" 2>&1 && less -R "$f"; rm -f "$f"`

	sh := exec.Command("sh", "-c", shCmd)
	return tea.ExecProcess(sh, func(err error) tea.Msg {
		if err != nil {
			return resourceActionStatusMsg{kind: "error", text: "describe failed: " + err.Error()}
		}
		return resourceActionStatusMsg{kind: "info", text: ""}
	})
}

// Breadcrumb returns the screen name for the nav bar.
func (s *ResourceListScreen) Breadcrumb() string {
	if s.action.Name != "" {
		return s.action.Name
	}
	return string(s.action.Kind)
}

// KeyHints returns per-screen contextual hints.
func (s *ResourceListScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{
		{Key: "⏎", Action: "details"},
		{Key: "r", Action: "refresh"},
		{Key: "?", Action: "help"},
	}
}

// Position satisfies PositionProvider so the footer can show n/total.
func (s *ResourceListScreen) Position() (int, int) { return s.picker.Position() }

// Update routes messages to the picker and handles picker events.
func (s *ResourceListScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		s.picker.SetSize(m.Width, pickerBodyHeight(m.Height))
		return s, nil

	case resourceListLoadedMsg:
		s.loading = false
		s.spinner.Stop()
		s.err = m.err
		if m.err != nil {
			return s, nil
		}
		items := make([]components.Item, 0, len(m.rows))
		for _, row := range m.rows {
			items = append(items, components.Item{
				Label:  row.label,
				Detail: row.detail,
				Meta:   row.meta,
			})
		}
		name := s.action.Name
		if name == "" {
			name = string(s.action.Kind)
		}
		binds := []components.Bind{{Key: "r", Action: "refresh"}}
		s.picker = components.New(name, items, binds)
		return s, nil

	case components.PickerSelectedMsg:
		if s.action.OnSelected != nil {
			return s, s.action.OnSelected(m.Value)
		}
		return s, defaultDescribe(s.action.Kind, s.action.Namespace, m.Value)

	case components.PickerActionMsg:
		if m.Action == "refresh" {
			s.loading = true
			s.err = nil
			s.status = ""
			return s, tea.Batch(s.loadResources(), s.spinner.Start())
		}
		return s, nil

	case components.PickerCancelMsg:
		s.spinner.Stop()
		return nil, nil

	case resourceActionStatusMsg:
		if m.text != "" {
			s.status = theme.InfoCallout(m.kind, m.text)
		}
		return s, nil
	}

	if cmd, handled := s.spinner.Update(msg); handled {
		return s, cmd
	}

	var cmd tea.Cmd
	s.picker, cmd = s.picker.Update(msg)
	return s, cmd
}

// View renders the resource list screen body.
func (s *ResourceListScreen) View(app *App) string {
	if s.loading {
		return "  " + s.spinner.View() + "  " + theme.Dim.Render("loading "+string(s.action.Kind)+"…")
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
