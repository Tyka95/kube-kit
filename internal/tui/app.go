// Package tui owns the top-level Bubble Tea program — the App model holds
// shared state (kube context, AWS session, breadcrumb stack, screen stack)
// and delegates per-screen behavior to the active screen.
package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/awssession"
	"github.com/Tyka95/kube-kit/internal/commands"
	"github.com/Tyka95/kube-kit/internal/config"
	"github.com/Tyka95/kube-kit/internal/kctx"
	"github.com/Tyka95/kube-kit/internal/rds"
	"github.com/Tyka95/kube-kit/internal/tui/components"
	"github.com/Tyka95/kube-kit/internal/tui/state"
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
func (a *App) Init() tea.Cmd {
	return tea.Batch(
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
		if m.String() == "ctrl+c" {
			return a, tea.Quit
		}
	case QuitMsg:
		return a, tea.Quit
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

	bodyHeight := a.Height - components.HeaderHeight - components.FooterHeight - 2 // -2 for breadcrumb + blank
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	body := a.Current().View(a)

	footer := components.Footer(a.AppState, components.FooterOpts{
		Position: a.currentPickerPosition(),
	})

	// Compose with absolute newlines so the screen lays out predictably.
	return header + "\n" + breadcrumb + "\n\n" + body + footer
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
