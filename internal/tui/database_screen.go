package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/awssession"
	"github.com/Tyka95/kube-kit/internal/config"
	"github.com/Tyka95/kube-kit/internal/kctx"
	"github.com/Tyka95/kube-kit/internal/rds"
	"github.com/Tyka95/kube-kit/internal/tunnel"
	"github.com/Tyka95/kube-kit/internal/tui/components"
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
	customEndpointLabel = "Custom endpoint"
	localTunnelPort     = 15432
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
	picker    components.Picker
	items     []components.Item
	endpoints []resolvedEndpoint // parallel to items; does NOT include the Custom row
	status    string             // "Discovering…", error, identity line, or mismatch warning
	tunnel    *tunnel.Tunnel
	tunnelMsg string // status line shown while a tunnel is active
}

// NewDatabaseScreen constructs a DatabaseScreen in the loading state.
func NewDatabaseScreen(cfg *config.Config, session *awssession.Session, discover *rds.Discoverer) *DatabaseScreen {
	return &DatabaseScreen{
		cfg:      cfg,
		session:  session,
		discover: discover,
		picker:   components.New("Database", nil, []components.Bind{{Key: "r", Action: "refresh"}}),
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
			s.items = append(s.items, components.Item{
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
			s.items = append(s.items, components.Item{
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
		s.items = append(s.items, components.Item{
			Label:  customEndpointLabel,
			Detail: "enter manually",
			Meta:   "",
		})

		s.picker = components.New("Database", s.items,
			[]components.Bind{{Key: "r", Action: "refresh"}})
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

	case components.PickerSelectedMsg:
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

		host := ep.Host
		port := ep.Port

		return s, func() tea.Msg {
			t, err := tunnel.Open(context.Background(), tunnel.Config{
				Namespace:  ns,
				Host:       host,
				RemotePort: port,
				LocalPort:  localTunnelPort,
			}, nil)
			if err != nil {
				// Surface the error in the status bar without changing screen.
				return dbTunnelErrMsg{err: err}
			}
			return dbTunnelOpenMsg{t: t, host: host, port: port}
		}

	case dbTunnelOpenMsg:
		s.tunnel = m.t
		s.tunnelMsg = fmt.Sprintf("tunnel active: localhost:%d → %s:%d",
			localTunnelPort, m.host, m.port)
		// Stay on this screen so the user sees the active tunnel.
		return s, nil

	case dbTunnelErrMsg:
		s.status = "tunnel error: " + m.err.Error()
		return s, nil

	case components.PickerActionMsg:
		if m.Action == "refresh" {
			s.discover.Invalidate()
			s.status = "Discovering…"
			return s, s.Init()
		}
		return s, nil

	case components.PickerCancelMsg:
		if s.tunnel != nil {
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

// View renders the database screen body.
func (s *DatabaseScreen) View(app *App) string {
	s.picker.SetSize(app.Width, pickerBodyHeight(app.Height))

	// Tunnel-active view: show connection info instead of the picker.
	if s.tunnel != nil {
		return theme.StatusOk.Render(s.tunnelMsg) + "\n" +
			theme.Dim.Render("press esc to disconnect")
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
	t    *tunnel.Tunnel
	host string
	port int
}

type dbTunnelErrMsg struct {
	err error
}
