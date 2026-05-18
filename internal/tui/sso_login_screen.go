package tui

import (
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tyka95/kube-kit/internal/awssession"
	"github.com/Tyka95/kube-kit/internal/tui/components"
	"github.com/Tyka95/kube-kit/internal/tui/state"
	"github.com/Tyka95/kube-kit/internal/tui/theme"
)

// SSOLoginScreen runs `aws sso login --no-browser --profile X` in the
// background, parses the device-code URL and verification code from its
// stdout, and renders them inside the kubekit chrome. The TUI never
// surrenders the terminal — the user opens the URL in any browser and
// confirms the code there. AWS CLI polls AWS until the device is
// authorized, then the screen reports success and the header refreshes.
type SSOLoginScreen struct {
	profile string
	session *awssession.Session
	spinner components.Spinner

	cmd    *exec.Cmd
	output *syncBuffer

	url            string
	code           string
	browserOpened  bool // guard so we only fire `open <url>` once
	codeCopied     bool // guard so we only copy to clipboard once
	clipboardOK    bool // true if last copy attempt succeeded

	completed bool
	finalKind string // "ok" | "warn" | "error" — drives the callout color
	finalMsg  string
}

// syncBuffer is a goroutine-safe bytes.Buffer used as the aws CLI's stdout
// sink. The TUI polls .String() every 250ms.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// Internal messages.
type ssoPollMsg struct{}
type ssoCompletedMsg struct{ err error }
type ssoValidatedMsg struct{ id awssession.Identity }

// AWS prints the device URL as a normal https link and the code on its own
// line. The two regexes accept the common formats.
var (
	ssoURLRe  = regexp.MustCompile(`https?://[a-zA-Z0-9.\-_/]+/start/#?/device[^\s]*`)
	ssoCodeRe = regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{4}\b`)
)

const ssoPollInterval = 250 * time.Millisecond

// NewSSOLoginScreen constructs a screen ready to run the SSO flow. Init()
// starts the aws CLI subprocess.
func NewSSOLoginScreen(profile string, session *awssession.Session) *SSOLoginScreen {
	return &SSOLoginScreen{
		profile: profile,
		session: session,
		spinner: components.NewSpinner(),
		output:  &syncBuffer{},
	}
}

func (s *SSOLoginScreen) Breadcrumb() string { return "SSO Login" }

func (s *SSOLoginScreen) KeyHints() []state.KeyHint {
	return []state.KeyHint{
		{Key: "esc", Action: "cancel"},
		{Key: "?", Action: "help"},
	}
}

// Position satisfies PositionProvider; this screen has no list, return 0/0.
func (s *SSOLoginScreen) Position() (int, int) { return 0, 0 }

// Init spawns `aws sso login --no-browser --profile X` and kicks off two
// commands in parallel: a poll cycle (drains output for URL+code) and a
// blocking wait on the subprocess (returns ssoCompletedMsg when done).
func (s *SSOLoginScreen) Init() tea.Cmd {
	s.cmd = exec.Command("aws", "sso", "login", "--no-browser", "--profile", s.profile)
	s.cmd.Stdout = s.output
	s.cmd.Stderr = s.output

	if err := s.cmd.Start(); err != nil {
		s.completed = true
		s.finalKind = "error"
		s.finalMsg = "failed to start aws sso login: " + err.Error()
		return nil
	}

	return tea.Batch(
		s.spinner.Start(),
		s.waitForCompletion(),
		s.tickPoll(),
	)
}

// waitForCompletion blocks on cmd.Wait() and returns ssoCompletedMsg with
// the exit error. Runs as a bubbletea Cmd (goroutine).
func (s *SSOLoginScreen) waitForCompletion() tea.Cmd {
	return func() tea.Msg {
		err := s.cmd.Wait()
		return ssoCompletedMsg{err: err}
	}
}

// tickPoll schedules the next output scan.
func (s *SSOLoginScreen) tickPoll() tea.Cmd {
	return tea.Tick(ssoPollInterval, func(time.Time) tea.Msg { return ssoPollMsg{} })
}

func (s *SSOLoginScreen) Update(msg tea.Msg, app *App) (Screen, tea.Cmd) {
	switch m := msg.(type) {

	case ssoPollMsg:
		s.scanOutput()
		// Auto-open the URL in the user's default browser exactly once.
		// --no-browser suppresses aws CLI's own attempt; we make the call.
		var openCmd tea.Cmd
		if s.url != "" && !s.browserOpened {
			s.browserOpened = true
			openCmd = openBrowserCmd(s.url)
		}
		// Auto-copy the verification code to the system clipboard exactly
		// once. The browser device-auth page expects the user to paste it.
		if s.code != "" && !s.codeCopied {
			s.codeCopied = true
			s.clipboardOK = clipboard.WriteAll(s.code) == nil
		}
		if s.completed {
			return s, openCmd
		}
		if openCmd == nil {
			return s, s.tickPoll()
		}
		return s, tea.Batch(openCmd, s.tickPoll())

	case ssoCompletedMsg:
		s.completed = true
		s.spinner.Stop()
		// Final scan in case the URL/code only landed in the last bytes.
		s.scanOutput()
		if m.err != nil {
			s.finalKind = "error"
			s.finalMsg = "login failed: " + m.err.Error()
			return s, nil
		}
		// Validate the resulting session so the header repaints.
		profile := s.profile
		session := s.session
		return s, func() tea.Msg {
			session.SetProfile(profile)
			id := session.Validate(context.Background(), true)
			return ssoValidatedMsg{id: id}
		}

	case ssoValidatedMsg:
		if m.id.Status != awssession.StatusOK {
			s.finalKind = "warn"
			s.finalMsg = "login finished but validate failed: " + m.id.Error
			return s, nil
		}
		s.finalKind = "ok"
		s.finalMsg = "logged in as " + s.profile + " (account " + m.id.Account + ")"
		// Refresh the header by re-emitting awsValidatedMsg to the app.
		return s, func() tea.Msg { return awsValidatedMsg{Identity: m.id} }

	case tea.KeyMsg:
		switch m.String() {
		case "esc", "q":
			s.cancelAndExit()
			return nil, nil
		case "enter":
			if s.completed {
				return nil, nil
			}
		}
	}

	// Spinner tick passes through.
	if cmd, handled := s.spinner.Update(msg); handled {
		return s, cmd
	}
	return s, nil
}

// scanOutput re-reads the captured stdout and fills in url/code if regex
// matches are found. Cheap to call repeatedly — the buffer grows append-only.
func (s *SSOLoginScreen) scanOutput() {
	if s.url != "" && s.code != "" {
		return
	}
	out := s.output.String()
	if s.url == "" {
		if u := ssoURLRe.FindString(out); u != "" {
			s.url = u
		}
	}
	if s.code == "" {
		if c := ssoCodeRe.FindString(out); c != "" {
			s.code = c
		}
	}
}

// cancelAndExit kills the aws subprocess so it doesn't keep polling AWS in
// the background after the user has dismissed the screen.
func (s *SSOLoginScreen) cancelAndExit() {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	if s.completed {
		return
	}
	_ = s.cmd.Process.Kill()
}

func (s *SSOLoginScreen) View(app *App) string {
	var b strings.Builder

	b.WriteString("  " + theme.HelpHeader.Render("AWS SSO Login"))
	b.WriteString("\n\n")
	b.WriteString("  " + theme.Dim.Render("Profile: ") + theme.HeaderValue.Render(s.profile))
	b.WriteString("\n\n")

	if s.completed {
		b.WriteString(theme.InfoCallout(s.finalKind, s.finalMsg))
		b.WriteString("\n\n  " + theme.Dim.Render("press esc or enter to return"))
		return b.String()
	}

	if s.url == "" {
		b.WriteString("  " + s.spinner.View() + "  " + theme.Dim.Render("starting aws sso login…"))
		return b.String()
	}

	b.WriteString("  " + theme.Dim.Render("Open this URL in any browser:"))
	b.WriteString("\n\n")
	b.WriteString("    " + theme.HeaderValue.Render(s.url))
	b.WriteString("\n\n")

	if s.code != "" {
		b.WriteString("  " + theme.Dim.Render("Enter this verification code:"))
		b.WriteString("\n\n")
		hint := "(copy failed — select manually)"
		if s.clipboardOK {
			hint = "(copied to clipboard)"
		}
		b.WriteString("    " + theme.HelpHeader.Render(s.code) + "  " + theme.Dim.Render(hint))
		b.WriteString("\n\n")
	}

	b.WriteString("  " + s.spinner.View() + "  " + theme.Dim.Render("waiting for browser confirmation…"))
	return b.String()
}

// openBrowserCmd returns a tea.Cmd that asks the OS to open url in the
// user's default browser. Best-effort: any failure is silently ignored
// since the URL is still visible on screen for the user to copy manually.
func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		_ = cmd.Start()
		return nil
	}
}

