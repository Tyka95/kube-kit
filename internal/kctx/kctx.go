// Package kctx provides helpers for reading and mutating the active kubectl
// context via the kubectl CLI (no client-go dependency).
package kctx

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// eksARNRe matches 'arn:aws:eks:<region>:<12-digit-account>:cluster/<name>'.
var eksARNRe = regexp.MustCompile(`^arn:aws:eks:([a-z0-9-]+):([0-9]{12}):cluster/(.+)$`)

// Context holds the parsed representation of a single kubectl context entry.
type Context struct {
	Name      string // raw context name (full ARN for EKS contexts)
	Cluster   string // short cluster name (last ARN segment), or = Name for non-EKS
	Region    string // empty for non-EKS
	Account   string // empty for non-EKS
	Namespace string // namespace pinned in this context, or "default"
	Current   bool   // true if this is the active context
}

// ParseEKSARN parses an EKS ARN of the form
// 'arn:aws:eks:<region>:<account>:cluster/<name>' and returns the three
// components. ok is false for any input that does not match.
func ParseEKSARN(s string) (region, account, cluster string, ok bool) {
	m := eksARNRe.FindStringSubmatch(s)
	if m == nil {
		return "", "", "", false
	}
	return m[1], m[2], m[3], true
}

// runKubectl executes kubectl with the given arguments and returns trimmed
// stdout. On a non-zero exit the error includes the first line of stderr.
func runKubectl(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrMsg := firstLine(stderr.String())
		if stderrMsg != "" {
			return "", fmt.Errorf("kubectl %s: %w: %s", strings.Join(args, " "), err, stderrMsg)
		}
		return "", fmt.Errorf("kubectl %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// firstLine returns the first non-empty line of s.
func firstLine(s string) string {
	for _, line := range strings.SplitN(s, "\n", -1) {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// CurrentContextName returns the active context name. For EKS clusters this is
// the full ARN.
func CurrentContextName(ctx context.Context) (string, error) {
	return runKubectl(ctx, "config", "current-context")
}

// CurrentNamespace returns the namespace pinned in the current context, or
// "default" when none is set.
func CurrentNamespace(ctx context.Context) (string, error) {
	ns, err := runKubectl(ctx, "config", "view", "--minify", "-o",
		"jsonpath={.contexts[0].context.namespace}")
	if err != nil {
		return "", err
	}
	if ns == "" {
		return "default", nil
	}
	return ns, nil
}

// SetNamespace pins ns to the current context via
// 'kubectl config set-context --current --namespace=<ns>'.
func SetNamespace(ctx context.Context, ns string) error {
	_, err := runKubectl(ctx, "config", "set-context", "--current",
		"--namespace="+ns)
	return err
}

// SwitchContext runs 'kubectl config use-context <name>'.
func SwitchContext(ctx context.Context, name string) error {
	_, err := runKubectl(ctx, "config", "use-context", name)
	return err
}

// ListContexts returns all configured contexts parsed into Context structs.
// The currently active context has Current=true. EKS ARN names are
// decomposed into Cluster/Region/Account; everything else has Cluster=Name
// with empty Region and Account. The Namespace field is populated from the
// kubeconfig for each context.
func ListContexts(ctx context.Context) ([]Context, error) {
	namesOut, err := runKubectl(ctx, "config", "get-contexts", "-o", "name")
	if err != nil {
		return nil, err
	}

	currentName, err := CurrentContextName(ctx)
	if err != nil {
		// Non-fatal: no current context configured yet.
		currentName = ""
	}

	var contexts []Context
	for _, name := range strings.Split(namesOut, "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		kctx := Context{Name: name}

		if region, account, cluster, ok := ParseEKSARN(name); ok {
			kctx.Region = region
			kctx.Account = account
			kctx.Cluster = cluster
		} else {
			kctx.Cluster = name
		}

		kctx.Current = name == currentName

		// Fetch the namespace pinned for this specific context.
		ns, nsErr := contextNamespace(ctx, name)
		if nsErr != nil {
			ns = "default"
		}
		kctx.Namespace = ns

		contexts = append(contexts, kctx)
	}
	return contexts, nil
}

// contextNamespace returns the namespace set for the named context, or
// "default" when none is set.
func contextNamespace(ctx context.Context, name string) (string, error) {
	// Use jsonpath to query the specific context by name.
	jsonpath := fmt.Sprintf(
		`jsonpath={.contexts[?(@.name=="%s")].context.namespace}`, name)
	ns, err := runKubectl(ctx, "config", "view", "-o", jsonpath)
	if err != nil {
		return "", err
	}
	if ns == "" {
		return "default", nil
	}
	return ns, nil
}

// CurrentContext returns a fully-populated Context for the active context.
func CurrentContext(ctx context.Context) (Context, error) {
	name, err := CurrentContextName(ctx)
	if err != nil {
		return Context{}, err
	}

	ns, err := CurrentNamespace(ctx)
	if err != nil {
		return Context{}, err
	}

	kctx := Context{
		Name:      name,
		Namespace: ns,
		Current:   true,
	}

	if region, account, cluster, ok := ParseEKSARN(name); ok {
		kctx.Region = region
		kctx.Account = account
		kctx.Cluster = cluster
	} else {
		kctx.Cluster = name
	}

	return kctx, nil
}
