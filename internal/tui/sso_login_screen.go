package tui

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"

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
	ptmx   *os.File // PTY master; nil if PTY allocation failed and we fell back to pipes
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

// AWS CLI prints two URLs with --no-browser: a bare verification URL
// (e.g. https://device.sso.us-east-1.amazonaws.com/) AND a
// verification_uri_complete that embeds ?user_code=XXXX-XXXX. We prefer
// the complete URL because the AWS page then pre-fills the code — the
// user just clicks Confirm, no paste step, no race with code expiry.
//
// The code regex matches the standalone "XXXX-XXXX" line AWS prints
// separately. It's anchored to line boundaries so it can't accidentally
// pick up a fragment embedded inside a URL or another token.
var (
	ssoURLAnyRe   = regexp.MustCompile(`https?://[^\s'"<>]+`)
	ssoCodeLineRe = regexp.MustCompile(`(?m)^\s*([A-Z0-9]{4}-[A-Z0-9]{4})\s*$`)
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
	// AWS CLI v2 is a PyInstaller-frozen Python binary. When stdout
	// isn't a TTY it switches to block-buffered output AND silences
	// the prompt_toolkit-based SSO prompt entirely, so piping stdout
	// into a Go io.Writer gives us nothing until the process exits.
	// PYTHONUNBUFFERED has no effect at this level.
	//
	// Fix: allocate a pseudo-terminal so aws thinks it's attached to
	// a real shell and prints the URL/code immediately. The PTY master
	// is read in a goroutine into our existing syncBuffer, so the rest
	// of the screen (regex scanning, browser open, clipboard) needs no
	// changes.
	// Pre-set a sane PTY size BEFORE starting. A 0×0 PTY makes
	// prompt_toolkit (used by aws CLI for SSO prompts) emit nothing,
	// which is the entire reason the screen used to sit silent.
	ptmx, err := pty.StartWithSize(s.cmd, &pty.Winsize{Rows: 40, Cols: 120})
	if err != nil {
		// PTY allocation failed (rare — usually only on locked-down
		// environments). Fall back to pipes; output likely won't be
		// visible, but we won't crash.
		s.cmd.Stdout = s.output
		s.cmd.Stderr = s.output
		s.cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
		if startErr := s.cmd.Start(); startErr != nil {
			s.completed = true
			s.finalKind = "error"
			s.finalMsg = "failed to start aws sso login: " + startErr.Error()
			return nil
		}
	} else {
		s.ptmx = ptmx
		// Drain the PTY master into our buffer. io.Copy returns when
		// the slave side closes (i.e. aws exits) — at which point the
		// waitForCompletion goroutine will already be observing the
		// process exit.
		go func() {
			buf := make([]byte, 4096)
			for {
				n, rerr := ptmx.Read(buf)
				if n > 0 {
					_, _ = s.output.Write(buf[:n])
				}
				if rerr != nil {
					return
				}
			}
		}()
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
//
// URL selection: AWS prints two URLs — a bare verification URL and a
// verification_uri_complete with ?user_code= embedded. We always prefer
// the complete one (page pre-fills the code → user only clicks Confirm).
func (s *SSOLoginScreen) scanOutput() {
	if s.url != "" && s.code != "" {
		return
	}
	out := s.output.String()
	if s.url == "" {
		urls := ssoURLAnyRe.FindAllString(out, -1)
		var bare string
		for _, u := range urls {
			if strings.Contains(u, "user_code=") {
				s.url = u
				break
			}
			if bare == "" {
				bare = u
			}
		}
		if s.url == "" {
			s.url = bare
		}
	}
	if s.code == "" {
		if m := ssoCodeLineRe.FindStringSubmatch(out); len(m) > 1 {
			s.code = m[1]
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
	if s.ptmx != nil {
		_ = s.ptmx.Close()
	}
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
		// Surface raw aws CLI output (stderr/stdout) while we wait, so
		// failures like the python@3.14 pyexpat dlopen error are visible
		// instead of sitting under a spinner forever.
		if raw := strings.TrimSpace(s.output.String()); raw != "" {
			b.WriteString("\n\n")
			for _, line := range strings.Split(raw, "\n") {
				b.WriteString("  " + theme.Dim.Render(line) + "\n")
			}
		}
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

