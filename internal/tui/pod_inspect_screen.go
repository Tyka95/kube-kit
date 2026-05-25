package tui

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/tui/components"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// PodInspectScreen renders `kubectl describe pod NAME -n NS` inside the
// kubekit chrome instead of suspending the TUI for a less subprocess.
// Old flow: tea.ExecProcess → kubectl describe | less → user trapped
// inside less's keybindings, no header/footer/breadcrumbs visible, only
// way out was less's `q` convention. New flow keeps the kubekit UI on
// screen with proper key hints and esc/q both work for exit.
type PodInspectScreen struct {
	pod       string
	namespace string

	vp       viewport.Model
	spinner  components.Spinner
	loading  bool
	err      string
	contents string

	// Wrapped to the viewport width on every WindowSizeMsg so we never
	// emit lines wider than the visible area (kubectl describe envs
	// often run very long).
	rawContents string
}

// podInspectLoadedMsg carries the kubectl describe output.
type podInspectLoadedMsg struct {
	output string
	err    error
}

// NewPodInspectScreen constructs a screen that fetches and renders
// `kubectl describe pod` inline.
func NewPodInspectScreen(pod, namespace string) *PodInspectScreen {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	return &PodInspectScreen{
		pod:       pod,
		namespace: namespace,
		vp:        vp,
		spinner:   components.NewSpinner(),
		loading:   true,
	}
}

// Breadcrumb pushes the pod name onto the trail.
func (s *PodInspectScreen) Breadcrumb() string {
	if len(s.pod) > 24 {
		return s.pod[:24] + "…"
	}
	return s.pod
}

// KeyHints lists the in-screen shortcuts. These also appear in the help overlay.
func (s *PodInspectScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{
		{Key: "↑↓", Action: "scroll"},
		{Key: "g/G", Action: "top/bottom"},
		{Key: "PgUp/PgDn", Action: "page"},
		{Key: "esc", Action: "back"},
		{Key: "?", Action: "help"},
	}
}

// Position satisfies PositionProvider — show scroll percentage.
func (s *PodInspectScreen) Position() (int, int) {
	if s.loading || s.contents == "" {
		return 0, 0
	}
	total := s.vp.TotalLineCount()
	if total == 0 {
		return 0, 0
	}
	// Approximate "current line" = YOffset + 1. Capped at total.
	cur := s.vp.YOffset + 1
	if cur > total {
		cur = total
	}
	return cur, total
}

// Init kicks off the kubectl describe fetch.
func (s *PodInspectScreen) Init() tea.Cmd {
	pod, ns := s.pod, s.namespace
	return tea.Batch(s.spinner.Start(), func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var out, stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, "kubectl", "describe", "pod", pod, "-n", ns)
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			msg := err.Error()
			if s := strings.TrimSpace(stderr.String()); s != "" {
				msg += ": " + s
			}
			return podInspectLoadedMsg{err: stringErr(msg)}
		}
		return podInspectLoadedMsg{output: out.String()}
	})
}

// Update handles load completion, key navigation, and resize.
func (s *PodInspectScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	switch m := msg.(type) {

	case podInspectLoadedMsg:
		s.loading = false
		if m.err != nil {
			s.err = m.err.Error()
			return s, nil
		}
		s.rawContents = m.output
		s.refreshViewport()
		return s, nil

	case tea.WindowSizeMsg:
		s.vp.Width = m.Width
		s.vp.Height = pickerBodyHeight(m.Height)
		s.refreshViewport()
		return s, nil

	case tea.KeyMsg:
		switch m.String() {
		case "esc", "q":
			return nil, nil
		}
	}

	// Spinner tick.
	if cmd, handled := s.spinner.Update(msg); handled {
		return s, cmd
	}

	// Forward everything else to the viewport so its built-in scroll
	// keys (j/k, g/G, PgUp/PgDn, mouse wheel, arrow keys) just work.
	var cmd tea.Cmd
	s.vp, cmd = s.vp.Update(msg)
	return s, cmd
}

// View renders the screen body.
func (s *PodInspectScreen) View(app *App) string {
	s.vp.Width = app.Width
	s.vp.Height = pickerBodyHeight(app.Height)

	if s.loading {
		return "  " + s.spinner.View() + "  " +
			theme.Dim.Render("loading: kubectl describe pod "+s.pod+" -n "+s.namespace+"…")
	}
	if s.err != "" {
		return theme.InfoCallout("error", "describe failed: "+s.err) +
			"\n\n  " + theme.Dim.Render("press esc to return")
	}
	return s.vp.View()
}

// refreshViewport re-renders the content into the viewport at the
// current width. Long unwrapped lines (env vars, annotations) get hard-
// wrapped to the viewport width so horizontal scrolling isn't needed.
func (s *PodInspectScreen) refreshViewport() {
	if s.rawContents == "" {
		return
	}
	width := s.vp.Width
	if width <= 4 {
		width = 80
	}
	s.contents = wrapLines(s.rawContents, width)
	s.vp.SetContent(s.contents)
	// Reset to top on first load or width change so the user sees
	// the head of the document, not a leftover offset.
	s.vp.GotoTop()
}

// wrapLines hard-wraps every line of s to maxWidth runes. Preserves
// blank lines. Used so kubectl describe's wide env-var lines don't
// trail off the right edge of the terminal.
func wrapLines(input string, maxWidth int) string {
	if maxWidth < 8 {
		maxWidth = 80
	}
	var out strings.Builder
	for i, line := range strings.Split(input, "\n") {
		if i > 0 {
			out.WriteByte('\n')
		}
		for len(line) > maxWidth {
			out.WriteString(line[:maxWidth])
			out.WriteByte('\n')
			line = line[maxWidth:]
		}
		out.WriteString(line)
	}
	return out.String()
}

// stringErr returns a tiny error implementation carrying a string. Used
// so the loaded-msg can shuttle a friendly message without dragging in
// the errors package.
type stringErr string

func (e stringErr) Error() string { return string(e) }
