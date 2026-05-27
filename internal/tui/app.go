// Package tui owns the top-level Bubble Tea program — the App model holds
// shared state (kube context, AWS session, breadcrumb stack, screen stack)
// and delegates per-screen behavior to the active screen.
package tui

import (
	"context"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/awssession"
	"github.com/Tyka95/kube-kit/internal/commands"
	"github.com/Tyka95/kube-kit/internal/config"
	"github.com/Tyka95/kube-kit/internal/kctx"
	"github.com/Tyka95/kube-kit/internal/rds"
	"github.com/Tyka95/kube-kit/internal/tui/components"
	"github.com/Tyka95/kube-kit/internal/tui/components/picker"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
	"github.com/Tyka95/kube-kit/internal/tunnel"
)

// Screen is implemented by every screen (main menu, database, aws, …).
type Screen interface {
	Init() tea.Cmd
	Update(msg tea.Msg, app *App) (Screen, tea.Cmd)
	View(app *App) string
	// Breadcrumb returns the name pushed onto the trail when this screen is active.
	Breadcrumb() string
	// KeyHints returns the per-screen contextual hints rendered in the header.
	KeyHints() []state.KeyHint
}

// App is the root Bubble Tea model.
type App struct {
	state.AppState
	screenStack []Screen

	// Shared singletons accessed by screens via App helpers.
	Config    *config.Config
	Session   *awssession.Session
	Discover  *rds.Discoverer
	Commands  *commands.Registry

	// Active tunnels — tracked here (not on the screen) so Cleanup can close
	// them on ANY exit path: Ctrl+C, :quit, SIGTERM, panic. Without this,
	// quitting the TUI orphans the kubectl port-forward subprocess locally
	// AND the socat pod in the cluster.
	tunnelMu sync.Mutex
	tunnels  []*tunnel.Tunnel
}

// NewApp constructs the app at the main menu.
func NewApp() *App {
	cfg, _ := config.Load() // empty config on error; the TUI is still usable
	if cfg == nil {
		cfg = &config.Config{}
	}
	a := &App{
		AppState: state.AppState{
			AWSStatus: state.AWSUnknown,
		},
		Config:   cfg,
		Session:  awssession.New(),
		Discover: rds.New(),
		Commands: commands.Builtins(),
	}
	a.Push(NewMainMenu())
	return a
}

// Push adds a screen on top of the stack and updates breadcrumbs / hints.
func (a *App) Push(s Screen) {
	a.screenStack = append(a.screenStack, s)
	if b := s.Breadcrumb(); b != "" {
		a.Breadcrumbs = append(a.Breadcrumbs, b)
	}
	a.KeyHints = s.KeyHints()
}

// Pop removes the top screen and reverts breadcrumb / hints.
func (a *App) Pop() {
	if len(a.screenStack) <= 1 {
		return
	}
	top := a.screenStack[len(a.screenStack)-1]
	a.screenStack = a.screenStack[:len(a.screenStack)-1]
	if top.Breadcrumb() != "" && len(a.Breadcrumbs) > 0 {
		a.Breadcrumbs = a.Breadcrumbs[:len(a.Breadcrumbs)-1]
	}
	a.KeyHints = a.screenStack[len(a.screenStack)-1].KeyHints()
}

// Current returns the active screen.
func (a *App) Current() Screen {
	return a.screenStack[len(a.screenStack)-1]
}

// Init delegates to the current screen and fires startup state-loaders.
// tea.HideCursor suppresses the terminal's blinking cursor so the only
// visible focus indicator is the picker's ❯ + selection bg.
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		tea.HideCursor,
		a.Current().Init(),
		a.loadKubeContext(),
		a.validateAWSSession(true),
	)
}

// ctxLoadedMsg carries the current kubectl context info.
type ctxLoadedMsg struct {
	Cluster   string
	Namespace string
	Err       error
}

// awsValidatedMsg carries the validated AWS identity snapshot.
type awsValidatedMsg struct {
	Identity awssession.Identity
}

// loadKubeContext shells out to kubectl once.
func (a *App) loadKubeContext() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		c, err := kctx.CurrentContext(ctx)
		if err != nil {
			return ctxLoadedMsg{Err: err}
		}
		return ctxLoadedMsg{Cluster: c.Cluster, Namespace: c.Namespace}
	}
}

// validateAWSSession runs sts get-caller-identity (with TTL guard if force=false).
func (a *App) validateAWSSession(force bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		id := a.Session.Validate(ctx, force)
		return awsValidatedMsg{Identity: id}
	}
}

// Update routes events.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.Width = m.Width
		a.Height = m.Height
		// fall through so the active screen sees the resize too.
	case tea.KeyMsg:
		// If the mismatch callout is active and the user presses 'a',
		// switch the AWS session to the suggested profile and revalidate.
		// Done before forwarding to the active screen so the keystroke
		// is unambiguous — screens never see 'a' while the callout is
		// up. Once accounts match, SuggestedAWSProfile clears and 'a'
		// flows through to screens normally.
		if m.String() == "a" && a.SuggestedAWSProfile != "" {
			a.Session.SetProfile(a.SuggestedAWSProfile)
			a.SuggestedAWSProfile = ""
			return a, a.validateAWSSession(true)
		}
		if m.String() == "ctrl+c" {
			// If a DB tunnel is up, Ctrl+C means "disconnect this tunnel"
			// — same intent as in a shell after `kubectl port-forward`.
			// Route it to the active screen as an Esc-equivalent so its
			// own close-with-spinner flow runs. Only quit the whole app
			// when there's no tunnel to disconnect.
			if len(a.tunnels) > 0 {
				escMsg := tea.KeyMsg{Type: tea.KeyEsc}
				cur, cmd := a.Current().Update(escMsg, a)
				if cur != nil {
					a.screenStack[len(a.screenStack)-1] = cur
				}
				return a, cmd
			}
			return a, tea.Quit
		}
	case QuitMsg:
		return a, tea.Quit
	case pushInspectMsg:
		// Pop the action picker we came from so esc-from-inspect lands
		// directly on the pod-action menu, not back on the same picker.
		a.Pop()
		a.Push(NewPodInspectScreen(m.pod, m.namespace))
		if initCmd := a.Current().Init(); initCmd != nil {
			return a, initCmd
		}
		return a, nil
	case picker.PickerHelpMsg:
		// Any screen's '?' lands here. Push the help overlay seeded with the
		// current screen's KeyHints so it can show them.
		a.Push(NewHelpScreen(a.KeyHints))
		return a, nil
	case picker.PickerCommandMsg:
		// Any screen's ':' lands here. Dispatch the command; on quit-sentinel
		// exit, on help-sentinel push help, otherwise just absorb (caller's
		// state is already updated by the handler).
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		res, err := a.Commands.Run(ctx, m.Value)
		if err == nil {
			switch res.Sentinel {
			case "quit":
				return a, tea.Quit
			case "help":
				a.Push(NewHelpScreen(a.KeyHints))
				return a, nil
			}
		}
		// If the command touched kube context / namespace / aws profile,
		// re-load state so the header reflects reality immediately.
		return a, tea.Batch(a.loadKubeContext(), a.validateAWSSession(true))
	case ctxLoadedMsg:
		if m.Err == nil {
			a.KubeContext = m.Cluster
			a.KubeNamespace = m.Namespace
		}
		return a, nil
	case awsValidatedMsg:
		id := m.Identity
		a.AWSProfile = id.Profile
		a.AWSAccount = id.Account
		a.AWSCtxAccount = id.CtxAccount
		a.AWSError = id.Error
		switch id.Status {
		case awssession.StatusOK:
			a.AWSStatus = state.AWSOK
		case awssession.StatusExpired:
			a.AWSStatus = state.AWSExpired
		case awssession.StatusNoAWS:
			a.AWSStatus = state.AWSNoAWS
		default:
			a.AWSStatus = state.AWSUnknown
		}
		// If the AWS session's account doesn't match the kubectl
		// context's account, see if ~/.aws/config has a profile whose
		// sso_account_id matches the cluster — if so, expose it as a
		// one-key 'switch' via the header callout. Cleared whenever
		// the accounts match (no callout needed when reconciled).
		a.SuggestedAWSProfile = ""
		if id.CtxAccount != "" && id.Account != "" && id.CtxAccount != id.Account {
			ctxAccount := id.CtxAccount
			return a, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				p := awssession.ProfileForAccount(ctx, ctxAccount)
				return suggestedProfileMsg{profile: p}
			}
		}
		return a, nil
	case suggestedProfileMsg:
		a.SuggestedAWSProfile = m.profile
		return a, nil
	}

	// Snapshot the stack length before the screen's Update so we can detect
	// in-Update navigation (a screen calling app.Push / app.Pop). Without this,
	// the post-call '[len-1] = cur' assignment would overwrite a freshly-pushed
	// child screen with the parent's return value.
	preLen := len(a.screenStack)
	preTop := a.Current()
	cur, cmd := preTop.Update(msg, a)
	postLen := len(a.screenStack)

	switch {
	case postLen > preLen:
		// The screen pushed a child during its update. cur belongs at the
		// screen's original position (which is now N-2, not the top).
		if cur != nil {
			a.screenStack[preLen-1] = cur
		}
		// Critical: invoke Init() on the newly-pushed screen so loaders fire.
		// Without this, screens like PodListScreen stay stuck on "loading…"
		// because kubectl never runs.
		newTop := a.screenStack[len(a.screenStack)-1]
		if initCmd := newTop.Init(); initCmd != nil {
			cmd = tea.Batch(cmd, initCmd)
		}
	case postLen < preLen:
		// Screen called app.Pop() directly. Nothing more to do.
	case cur == nil:
		// Self-pop via nil return.
		a.Pop()
	default:
		a.screenStack[postLen-1] = cur
	}
	return a, cmd
}

// View composes chrome + active screen body.
func (a *App) View() string {
	if a.Width == 0 || a.Height == 0 {
		return ""
	}

	header := components.Header(a.AppState)
	breadcrumb := components.Breadcrumb(a.AppState)
	body := a.Current().View(a)
	footer := components.Footer(a.AppState, components.FooterOpts{
		Position: a.currentPickerPosition(),
	})

	// Mismatch callout — rendered between breadcrumb and body when the
	// active AWS session targets a different account than the kube
	// context AND ~/.aws/config has a profile that would reconcile it.
	// One keypress (`a`) flips the session.
	var callout string
	if a.SuggestedAWSProfile != "" {
		callout = theme.InfoCallout("warn", "cluster needs account "+
			a.AWSCtxAccount+"  ·  press 'a' to switch profile to "+
			a.SuggestedAWSProfile)
	}

	// Compute how many rows the body actually occupies, then pad with blank
	// lines so the footer is pinned to the bottom of the terminal regardless
	// of how short the body is (e.g. 'loading pods…' that's only 1 line).
	headerRows := strings.Count(header, "\n") + 1
	breadcrumbRows := 1
	footerRows := strings.Count(footer, "\n") + 1
	calloutRows := 0
	if callout != "" {
		calloutRows = strings.Count(callout, "\n") + 1 + 1 // + trailing blank
	}
	// Layout: header + blank + breadcrumb + (callout + blank)? + blank + body + (pad) + footer.
	usedFixed := headerRows + 1 + breadcrumbRows + calloutRows + 1 + footerRows
	bodyLines := strings.Count(body, "\n")
	pad := a.Height - usedFixed - bodyLines
	if pad < 0 {
		pad = 0
	}
	padding := strings.Repeat("\n", pad)

	out := header + "\n" + breadcrumb + "\n"
	if callout != "" {
		out += callout + "\n\n"
	} else {
		out += "\n"
	}
	out += body + padding + footer
	return out
}

// currentPickerPosition asks the active screen if it has a position to show.
func (a *App) currentPickerPosition() string {
	if pp, ok := a.Current().(PositionProvider); ok {
		n, m := pp.Position()
		return components.MakePosition(n, m)
	}
	return ""
}

// PositionProvider lets screens expose a 1-based picker position.
type PositionProvider interface {
	Position() (n, total int)
}

// QuitMsg is broadcast when the app should exit.
type QuitMsg struct{}

// suggestedProfileMsg carries the result of the async ~/.aws/config
// scan triggered after a mismatch is detected. Empty Profile means no
// matching profile was found — the callout stays hidden.
type suggestedProfileMsg struct {
	profile string
}

// pushInspectMsg is emitted by the Pods screen when the user picks a
// pod under the Inspect action. App.Update catches it here and pushes
// the PodInspectScreen — done at this layer because the leaf callback
// inside PodActionScreen has no reference to *App.
type pushInspectMsg struct {
	pod       string
	namespace string
}

// AddTunnel registers an active tunnel for cleanup on exit. Screens call
// this immediately after a successful tunnel.Open so Ctrl+C / SIGTERM /
// panic-recovery paths can all tear it down. Idempotent for nil.
func (a *App) AddTunnel(t *tunnel.Tunnel) {
	if t == nil {
		return
	}
	a.tunnelMu.Lock()
	defer a.tunnelMu.Unlock()
	a.tunnels = append(a.tunnels, t)
}

// RemoveTunnel deregisters a tunnel that the caller has already closed.
// Called from the Database screen's normal-flow disconnect so Cleanup
// doesn't try to close it again (which would just be a noop, but keeps
// the slice from growing across multiple open/close cycles).
func (a *App) RemoveTunnel(t *tunnel.Tunnel) {
	if t == nil {
		return
	}
	a.tunnelMu.Lock()
	defer a.tunnelMu.Unlock()
	for i, x := range a.tunnels {
		if x == t {
			a.tunnels = append(a.tunnels[:i], a.tunnels[i+1:]...)
			return
		}
	}
}

// Cleanup closes all active tunnels. Safe to call multiple times — each
// tunnel.Close is idempotent. main.go defers this so it fires on every
// exit path (graceful quit, Ctrl+C, signal, error return).
func (a *App) Cleanup() {
	a.tunnelMu.Lock()
	defer a.tunnelMu.Unlock()
	for _, t := range a.tunnels {
		_ = t.Close()
	}
	a.tunnels = nil
}

