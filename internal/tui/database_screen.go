package tui

import (
	"context"
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/awssession"
	"github.com/Tyka95/kube-kit/internal/config"
	"github.com/Tyka95/kube-kit/internal/kctx"
	"github.com/Tyka95/kube-kit/internal/rds"
	"github.com/Tyka95/kube-kit/internal/tunnel"
	"github.com/Tyka95/kube-kit/internal/tui/components/picker"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// dbDataLoadedMsg is dispatched when the async discovery completes.
type dbDataLoadedMsg struct {
	snap      awssession.Identity
	endpoints []rds.Endpoint
	err       error
}

// customEndpointPickedMsg is dispatched when the user selects the "Custom endpoint" stub.
// TODO(v1.1): open a text-input dialog to accept an arbitrary host:port.
type customEndpointPickedMsg struct{}

const (
	customEndpointLabel    = "Custom endpoint"
	defaultLocalTunnelPort = 15432
)

// resolvedEndpoint carries the full connection info for a picker row.
type resolvedEndpoint struct {
	Host string
	Port int
	Kind string // "configured" | "discovered" | "manual"
	Tag  string // profile tag for discovered, or "current" for configured
}

// DatabaseScreen lists RDS / Aurora endpoints and lets the user open a tunnel.
type DatabaseScreen struct {
	cfg       *config.Config
	session   *awssession.Session
	discover  *rds.Discoverer
	picker    picker.Picker
	items     []picker.Item
	endpoints []resolvedEndpoint // parallel to items; does NOT include the Custom row
	status    string             // "Discovering…", error, identity line, or mismatch warning
	tunnel    *tunnel.Tunnel
	tunnelMsg string // status line shown while a tunnel is active

	// Local-port prompt state. Activated after the user picks an endpoint;
	// blocks until the user submits a port (or Esc to cancel). Pending* hold
	// the chosen endpoint and namespace while we wait for the port input.
	portPrompt    bool
	portInput     string
	portErr       string
	pendingHost   string
	pendingPort   int
	pendingNS     string
	pendingLabel  string
}

// NewDatabaseScreen constructs a DatabaseScreen in the loading state.
func NewDatabaseScreen(cfg *config.Config, session *awssession.Session, discover *rds.Discoverer) *DatabaseScreen {
	return &DatabaseScreen{
		cfg:      cfg,
		session:  session,
		discover: discover,
		picker:   picker.New("Database", nil, []picker.Bind{{Key: "r", Action: "refresh"}}),
		status:   "Discovering…",
	}
}

// Init fires the async session-ensure + RDS-discover call.
func (s *DatabaseScreen) Init() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		snap, err := s.session.Ensure(ctx)
		if err != nil {
			return dbDataLoadedMsg{snap: snap, err: err}
		}

		endpoints, discErr := s.discover.Discover(ctx, snap.Profile, snap.Region, snap.Account)
		return dbDataLoadedMsg{snap: snap, endpoints: endpoints, err: discErr}
	}
}

// Breadcrumb returns the name pushed onto the trail.
func (s *DatabaseScreen) Breadcrumb() string { return "Database" }

// KeyHints returns the per-screen contextual hints.
func (s *DatabaseScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{
		{Key: "⏎", Action: "tunnel"},
		{Key: "r", Action: "refresh"},
		{Key: "?", Action: "help"},
	}
}

// Position satisfies PositionProvider so the footer can show n/total.
func (s *DatabaseScreen) Position() (int, int) { return s.picker.Position() }

// Update routes messages to the picker and handles picker/db events.
func (s *DatabaseScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	// Port-prompt mode swallows all key input until the user submits
	// or cancels. Anything else (window resizes, tunnel result messages)
	// still falls through to the normal handlers below.
	if s.portPrompt {
		if km, ok := msg.(tea.KeyMsg); ok {
			return s.handlePortPromptKey(km)
		}
	}

	switch m := msg.(type) {

	case dbDataLoadedMsg:
		if m.err != nil {
			s.status = "Discovery error: " + m.err.Error()
			return s, nil
		}

		// Build picker items, deduped by host.
		seen := make(map[string]bool)
		s.items = s.items[:0]
		s.endpoints = s.endpoints[:0]

		// Configured targets come first.
		for _, t := range s.cfg.DBTargets {
			if seen[t.Host] {
				continue
			}
			seen[t.Host] = true
			s.items = append(s.items, picker.Item{
				Label:  t.Name,
				Detail: "configured",
				Meta:   "",
			})
			s.endpoints = append(s.endpoints, resolvedEndpoint{
				Host: t.Host,
				Port: t.Port,
				Kind: "configured",
				Tag:  "current",
			})
		}

		// Discovered endpoints.
		for _, ep := range m.endpoints {
			if seen[ep.Host] {
				continue
			}
			seen[ep.Host] = true
			s.items = append(s.items, picker.Item{
				Label:  ep.Identifier,
				Detail: ep.Region,
				Meta:   ep.Profile,
			})
			s.endpoints = append(s.endpoints, resolvedEndpoint{
				Host: ep.Host,
				Port: ep.Port,
				Kind: "discovered",
				Tag:  ep.Profile,
			})
		}

		// Custom endpoint stub — always last; parallel endpoints slice does NOT
		// hold an entry for this row (handled separately in PickerSelectedMsg).
		s.items = append(s.items, picker.Item{
			Label:  customEndpointLabel,
			Detail: "enter manually",
			Meta:   "",
		})

		s.picker = picker.New("Database", s.items,
			[]picker.Bind{{Key: "r", Action: "refresh"}})
		s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))

		// Build the identity status line.
		snap := m.snap
		if snap.CtxAccount != "" && snap.Account != snap.CtxAccount {
			s.status = fmt.Sprintf(
				"⚠  AWS account %s doesn't match cluster account %s",
				snap.Account, snap.CtxAccount,
			)
		} else {
			identity := snap.Profile
			if snap.Account != "" {
				if identity != "" {
					identity += "  "
				}
				identity += snap.Account
			}
			if snap.Region != "" {
				if identity != "" {
					identity += "  "
				}
				identity += snap.Region
			}
			s.status = identity
		}
		return s, nil

	case picker.PickerSelectedMsg:
		// Custom endpoint stub.
		if m.Value == customEndpointLabel {
			// TODO(v1.1): open an inline text-input dialog for arbitrary host:port.
			return s, func() tea.Msg { return customEndpointPickedMsg{} }
		}

		// Find the matching resolvedEndpoint by label.
		var ep resolvedEndpoint
		for i, it := range s.items {
			if it.Label == m.Value && i < len(s.endpoints) {
				ep = s.endpoints[i]
				break
			}
		}
		if ep.Host == "" {
			return s, nil
		}

		// Resolve the kubernetes namespace: prefer app state, fall back to kctx.
		ns := app.KubeNamespace
		if ns == "" {
			ns, _ = kctx.CurrentNamespace(context.Background())
		}
		if ns == "" {
			ns = "default"
		}

		// Stash the endpoint and open the local-port prompt. The actual
		// tunnel.Open call happens when the user submits the prompt.
		s.portPrompt = true
		s.portInput = ""
		s.portErr = ""
		s.pendingHost = ep.Host
		s.pendingPort = ep.Port
		s.pendingNS = ns
		s.pendingLabel = m.Value
		return s, nil

	case dbTunnelOpenMsg:
		s.tunnel = m.t
		s.tunnelMsg = fmt.Sprintf("tunnel active: localhost:%d → %s:%d",
			m.localPort, m.host, m.port)
		// Register with the app so Cleanup() can close the tunnel on any
		// exit path (Ctrl+C, signal, panic). Without this, quitting orphans
		// the kubectl port-forward AND the socat pod in the cluster.
		app.AddTunnel(m.t)
		// Stay on this screen so the user sees the active tunnel.
		return s, nil

	case dbTunnelErrMsg:
		s.status = "tunnel error: " + m.err.Error()
		return s, nil

	case picker.PickerActionMsg:
		if m.Action == "refresh" {
			s.discover.Invalidate()
			s.status = "Discovering…"
			return s, s.Init()
		}
		return s, nil

	case picker.PickerCancelMsg:
		if s.tunnel != nil {
			app.RemoveTunnel(s.tunnel)
			_ = s.tunnel.Close()
			s.tunnel = nil
			s.tunnelMsg = ""
			return s, nil
		}
		// Self-pop.
		return nil, nil

	case tea.WindowSizeMsg:
		s.picker.SetSize(m.Width, pickerBodyHeight(m.Height))
		return s, nil
	}

	var cmd tea.Cmd
	s.picker, cmd = s.picker.Update(msg)
	return s, cmd
}

// handlePortPromptKey processes keystrokes while the local-port prompt is
// active. Digits build the input, backspace deletes, enter submits (empty =
// default port), esc cancels back to the picker.
func (s *DatabaseScreen) handlePortPromptKey(km tea.KeyMsg) (Screen, tea.Cmd) {
	switch km.String() {
	case "esc":
		s.resetPortPrompt()
		return s, nil
	case "enter":
		port := defaultLocalTunnelPort
		if s.portInput != "" {
			p, err := strconv.Atoi(s.portInput)
			if err != nil || p < 1 || p > 65535 {
				s.portErr = "port must be 1-65535"
				return s, nil
			}
			port = p
		}
		host, remote, ns := s.pendingHost, s.pendingPort, s.pendingNS
		s.resetPortPrompt()
		return s, func() tea.Msg {
			t, err := tunnel.Open(context.Background(), tunnel.Config{
				Namespace:  ns,
				Host:       host,
				RemotePort: remote,
				LocalPort:  port,
			}, nil)
			if err != nil {
				return dbTunnelErrMsg{err: err}
			}
			return dbTunnelOpenMsg{t: t, host: host, port: remote, localPort: port}
		}
	case "backspace":
		if n := len(s.portInput); n > 0 {
			s.portInput = s.portInput[:n-1]
			s.portErr = ""
		}
		return s, nil
	}
	// Accept digits only.
	if r := km.Runes; len(r) == 1 && r[0] >= '0' && r[0] <= '9' && len(s.portInput) < 5 {
		s.portInput += string(r)
		s.portErr = ""
	}
	return s, nil
}

// resetPortPrompt clears the prompt state without opening a tunnel.
func (s *DatabaseScreen) resetPortPrompt() {
	s.portPrompt = false
	s.portInput = ""
	s.portErr = ""
	s.pendingHost = ""
	s.pendingPort = 0
	s.pendingNS = ""
	s.pendingLabel = ""
}

// View renders the database screen body.
func (s *DatabaseScreen) View(app *App) string {
	s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))

	// Tunnel-active view: show connection info instead of the picker.
	if s.tunnel != nil {
		return theme.StatusOk.Render(s.tunnelMsg) + "\n" +
			theme.Dim.Render("press esc to disconnect")
	}

	// Port-prompt view: ask for the local port before opening the tunnel.
	if s.portPrompt {
		shown := s.portInput
		if shown == "" {
			shown = strconv.Itoa(defaultLocalTunnelPort)
		}
		body := theme.Dim.Render("Open tunnel to ") +
			theme.HeaderValue.Render(s.pendingLabel) +
			"\n\n" +
			theme.Dim.Render("Local port: ") +
			theme.HeaderValue.Render(shown) +
			theme.Dim.Render("█")
		if s.portInput == "" {
			body += "  " + theme.Dim.Render("(default)")
		}
		hint := theme.Dim.Render("enter to open · esc to cancel · digits only")
		if s.portErr != "" {
			hint = theme.StatusWarn.Render(s.portErr) + "  " + hint
		}
		return body + "\n\n" + hint
	}

	// Determine status style: warn on account mismatch, dim otherwise.
	snap := s.session.Snapshot()
	var statusLine string
	if snap.CtxAccount != "" && snap.Account != snap.CtxAccount {
		statusLine = theme.StatusWarn.Render(s.status)
	} else {
		statusLine = theme.Dim.Render(s.status)
	}

	return statusLine + "\n\n" + s.picker.View()
}

// ── internal tunnel result messages ──────────────────────────────────────────

type dbTunnelOpenMsg struct {
	t         *tunnel.Tunnel
	host      string
	port      int
	localPort int
}

type dbTunnelErrMsg struct {
	err error
}
