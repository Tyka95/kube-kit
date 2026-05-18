// Package awssession is the single source of truth for AWS identity used by
// every AWS-aware action in kube-kit. All AWS interactions shell out to the
// aws CLI — no SDK dependency.
package awssession

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Status represents the current state of the AWS session.
type Status string

const (
	StatusUnknown Status = "unknown"
	StatusOK      Status = "ok"
	StatusExpired Status = "expired"
	StatusNoAWS   Status = "no-aws"
)

// ttl is the minimum interval between sts get-caller-identity calls when the
// last check was successful.
const ttl = 60 * time.Second

// eksARNRe matches an EKS cluster ARN and captures region and account id.
var eksARNRe = regexp.MustCompile(`arn:aws:eks:([a-z0-9-]+):([0-9]{12}):`)

// eksServerRe matches an EKS cluster API server URL of the form
// https://<id>.<x>.<region>.eks.amazonaws.com (or variant). Captures region.
var eksServerRe = regexp.MustCompile(`https://[^.]+\.[^.]+\.([a-z0-9-]+)\.eks\.amazonaws\.com`)

// Sentinel errors.
var (
	ErrSessionExpired = errors.New("aws session expired")
	ErrNoAWS          = errors.New("aws cli not available or no profile configured")
)

// Identity is a snapshot of the validated AWS session.
type Identity struct {
	Profile    string
	Region     string
	Account    string // 12-digit AWS account id
	ARN        string
	Status     Status
	Error      string    // first line of stderr from sts if failed
	CheckedAt  time.Time
	CtxAccount string // account id extracted from current kubectl EKS context ARN, if any
}

// Session holds the cached AWS identity and the mutex that guards it.
type Session struct {
	mu       sync.Mutex
	identity Identity
}

// New returns an initialised Session with Status=StatusUnknown.
func New() *Session {
	return &Session{
		identity: Identity{Status: StatusUnknown},
	}
}

// Snapshot returns a copy of the current cached Identity without doing any work.
func (s *Session) Snapshot() Identity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.identity
}

// SetProfile overrides the resolved profile. The next Validate call runs
// against this profile.
func (s *Session) SetProfile(profile string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.identity.Profile = profile
}

// Resolve recomputes Profile and Region from environment (kubeconfig exec env,
// $AWS_PROFILE, $AWS_DEFAULT_PROFILE, $AWS_REGION, EKS context ARN).
// Does not call AWS.
func (s *Session) Resolve(ctx context.Context) Identity {
	profile := resolveProfile(ctx)
	region, ctxAccount := resolveRegionAndAccount(ctx)

	s.mu.Lock()
	s.identity.Profile = profile
	s.identity.Region = region
	s.identity.CtxAccount = ctxAccount
	snap := s.identity
	s.mu.Unlock()
	return snap
}

// Validate runs `aws sts get-caller-identity` and updates the cached Identity.
// 60s TTL: if the last successful check was within 60s, returns the cached
// value unless force=true.
//
// Region and CtxAccount are re-resolved from the environment on each force
// call so a kubectl context switch in another terminal becomes visible.
// Profile is only resolved when it's currently empty — callers who explicitly
// SetProfile() (e.g. after SSO login) keep their override.
func (s *Session) Validate(ctx context.Context, force bool) Identity {
	s.mu.Lock()
	id := s.identity
	s.mu.Unlock()

	if !force && id.Status == StatusOK && time.Since(id.CheckedAt) < ttl {
		return id
	}

	// Refresh region/ctxAccount; refresh profile only if currently unset.
	if force {
		region, ctxAccount := resolveRegionAndAccount(ctx)
		s.mu.Lock()
		s.identity.Region = region
		s.identity.CtxAccount = ctxAccount
		if s.identity.Profile == "" {
			s.identity.Profile = resolveProfile(ctx)
		}
		s.mu.Unlock()
	}

	// Check aws CLI is available.
	if _, err := exec.LookPath("aws"); err != nil {
		s.mu.Lock()
		s.identity.Status = StatusNoAWS
		s.identity.Account = ""
		s.identity.ARN = ""
		s.identity.Error = "aws cli not installed"
		snap := s.identity
		s.mu.Unlock()
		return snap
	}

	s.mu.Lock()
	profile := s.identity.Profile
	region := s.identity.Region
	s.mu.Unlock()

	args := []string{"sts", "get-caller-identity",
		"--output", "text",
		"--query", "[Account,Arn]",
		"--cli-connect-timeout", "3",
		"--cli-read-timeout", "5",
	}
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	if region != "" {
		args = append(args, "--region", region)
	}

	cmd := exec.CommandContext(ctx, "aws", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	s.mu.Lock()
	defer s.mu.Unlock()

	out := strings.TrimSpace(stdout.String())
	if runErr == nil && out != "" {
		parts := strings.SplitN(out, "\t", 2)
		account := parts[0]
		arn := ""
		if len(parts) == 2 {
			arn = parts[1]
		}
		s.identity.Account = account
		s.identity.ARN = arn
		s.identity.Status = StatusOK
		s.identity.Error = ""
		s.identity.CheckedAt = time.Now()
		return s.identity
	}

	// Failed — classify by stderr.
	errLine := firstLine(stderr.String())
	s.identity.Account = ""
	s.identity.ARN = ""
	s.identity.Error = errLine
	if errLine == "" {
		s.identity.Error = "sts call failed"
	}

	switch {
	case containsAny(errLine,
		"ExpiredToken", "InvalidClientTokenId",
		"Token has expired", "SSO session",
		"sso-oidc", "refresh"):
		s.identity.Status = StatusExpired
	case profile == "" && os.Getenv("AWS_ACCESS_KEY_ID") == "":
		s.identity.Status = StatusNoAWS
	default:
		s.identity.Status = StatusUnknown
	}
	return s.identity
}

// Login runs `aws sso login --profile <profile>` interactively (inherits
// stdin/stdout so the user can complete the browser flow), then re-validates.
func (s *Session) Login(ctx context.Context) (Identity, error) {
	if _, err := exec.LookPath("aws"); err != nil {
		return s.Snapshot(), ErrNoAWS
	}

	s.mu.Lock()
	profile := s.identity.Profile
	s.mu.Unlock()

	if profile == "" {
		return s.Snapshot(), errors.New("no profile to log in with")
	}

	cmd := exec.CommandContext(ctx, "aws", "sso", "login", "--profile", profile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Non-fatal: re-validate regardless and surface any error via Identity.
		_ = err
	}

	id := s.Validate(ctx, true)
	return id, nil
}

// Ensure validates (with TTL); if Status==Expired it returns ErrSessionExpired,
// if Status==NoAWS it returns ErrNoAWS, if Status==OK it returns nil.
func (s *Session) Ensure(ctx context.Context) (Identity, error) {
	id := s.Validate(ctx, false)
	switch id.Status {
	case StatusOK:
		return id, nil
	case StatusExpired:
		return id, ErrSessionExpired
	case StatusNoAWS:
		return id, ErrNoAWS
	default:
		return id, errors.New(id.Error)
	}
}

// ContextChanged invalidates the cached identity and re-resolves. Call after a
// kubectl context switch or any other event that may change identity.
func (s *Session) ContextChanged(ctx context.Context) {
	s.mu.Lock()
	s.identity.Status = StatusUnknown
	s.identity.Account = ""
	s.identity.ARN = ""
	s.identity.CheckedAt = time.Time{}
	s.mu.Unlock()

	s.Resolve(ctx)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// resolveProfile derives the AWS profile from kubeconfig exec env, then
// $AWS_PROFILE, then $AWS_DEFAULT_PROFILE.
func resolveProfile(ctx context.Context) string {
	// Try kubeconfig exec env first.
	if p := kubeconfigExecEnvProfile(ctx); p != "" {
		return p
	}
	if p := os.Getenv("AWS_PROFILE"); p != "" {
		return p
	}
	return os.Getenv("AWS_DEFAULT_PROFILE")
}

// kubeconfigExecEnvProfile calls kubectl to extract the AWS_PROFILE value from
// the active kubeconfig user's exec environment. Returns empty string on any
// failure (kubectl missing, no exec env, etc.).
func kubeconfigExecEnvProfile(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "kubectl", "config", "view",
		"--minify", "-o",
		`jsonpath={.users[0].user.exec.env[?(@.name=="AWS_PROFILE")].value}`)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// resolveRegionAndAccount derives region and context account from:
//  1. the current kubectl context name (when it's an EKS ARN),
//  2. the cluster's API server URL in the kubeconfig (when the context is
//     a short alias like "staging-eks" but the cluster URL still ends in
//     <region>.eks.amazonaws.com),
//  3. $AWS_REGION / $AWS_DEFAULT_REGION as a last resort.
//
// CtxAccount can only be recovered from path (1) — the server URL doesn't
// encode the AWS account id.
func resolveRegionAndAccount(ctx context.Context) (region, ctxAccount string) {
	// (1) current-context as ARN.
	if out, err := exec.CommandContext(ctx, "kubectl", "config", "current-context").Output(); err == nil {
		ctxName := strings.TrimSpace(string(out))
		if m := eksARNRe.FindStringSubmatch(ctxName); m != nil {
			return m[1], m[2]
		}
	}

	// (2) cluster server URL from minified kubeconfig.
	if out, err := exec.CommandContext(ctx, "kubectl", "config", "view", "--minify",
		"-o", "jsonpath={.clusters[0].cluster.server}").Output(); err == nil {
		if m := eksServerRe.FindStringSubmatch(string(out)); m != nil {
			return m[1], ""
		}
	}

	// (3) env fallbacks.
	if r := os.Getenv("AWS_REGION"); r != "" {
		return r, ""
	}
	return os.Getenv("AWS_DEFAULT_REGION"), ""
}

// firstLine returns the first non-empty line of s.
func firstLine(s string) string {
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			return line
		}
	}
	return ""
}

// containsAny reports whether s contains any of the provided substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
