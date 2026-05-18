// Package commands implements the ":" command palette that maps short command
// names (e.g. ":ns kube-system") to handler functions. It is intentionally
// free of cross-package dependencies so that it can be used as a pure value
// type by any layer of the TUI.
package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Command is one registered command (e.g. ":ns kube-system").
type Command struct {
	Name        string
	Description string
	Handler     func(ctx context.Context, args string) (Result, error)
}

// Result is what a handler returns. The TUI app intercepts the Sentinel field
// for special actions like quit / help, otherwise renders Message.
type Result struct {
	Sentinel string // "" | "quit" | "help"
	Message  string // shown in the footer or as a toast
}

// Registry holds a set of named commands, preserving insertion order.
type Registry struct {
	cmds  map[string]Command
	order []string // tracks insertion order; duplicates replaced in-place
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		cmds:  make(map[string]Command),
		order: []string{},
	}
}

// Register adds a command. Duplicate names overwrite the previous entry but
// keep their slot in the order slice.
func (r *Registry) Register(c Command) {
	if _, exists := r.cmds[c.Name]; !exists {
		r.order = append(r.order, c.Name)
	}
	r.cmds[c.Name] = c
}

// Run parses 'name arg1 arg2 ...' from input. Empty input → error "no command".
// Unknown name → error "unknown command: X". Otherwise calls Handler.
func (r *Registry) Run(ctx context.Context, input string) (Result, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return Result{}, fmt.Errorf("no command")
	}

	// Strip a leading colon if present (e.g. ":ns kube-system").
	if strings.HasPrefix(input, ":") {
		input = input[1:]
	}

	var name, args string
	if idx := strings.Index(input, " "); idx >= 0 {
		name = input[:idx]
		args = strings.TrimSpace(input[idx+1:])
	} else {
		name = input
		args = ""
	}

	c, ok := r.cmds[name]
	if !ok {
		return Result{}, fmt.Errorf("unknown command: %s", name)
	}

	return c.Handler(ctx, args)
}

// List returns commands in registration order. Used by the help overlay.
func (r *Registry) List() []Command {
	out := make([]Command, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.cmds[name])
	}
	return out
}

// Builtins returns a Registry pre-populated with the standard commands.
func Builtins() *Registry {
	r := NewRegistry()

	r.Register(Command{
		Name:        "q",
		Description: "quit",
		Handler: func(_ context.Context, _ string) (Result, error) {
			return Result{Sentinel: "quit"}, nil
		},
	})

	r.Register(Command{
		Name:        "quit",
		Description: "quit",
		Handler: func(_ context.Context, _ string) (Result, error) {
			return Result{Sentinel: "quit"}, nil
		},
	})

	r.Register(Command{
		Name:        "help",
		Description: "show help overlay",
		Handler: func(_ context.Context, _ string) (Result, error) {
			return Result{Sentinel: "help"}, nil
		},
	})

	r.Register(Command{
		Name:        "ns",
		Description: "set namespace",
		Handler: func(ctx context.Context, args string) (Result, error) {
			ns := strings.TrimSpace(args)
			if ns == "" {
				return Result{}, fmt.Errorf("ns: missing argument")
			}
			cmd := exec.CommandContext(ctx,
				"kubectl", "config", "set-context", "--current",
				"--namespace="+ns,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				return Result{}, fmt.Errorf("ns: %w: %s", err, strings.TrimSpace(string(out)))
			}
			return Result{Message: "namespace → " + ns}, nil
		},
	})

	r.Register(Command{
		Name:        "context",
		Description: "switch kube ctx",
		Handler: func(ctx context.Context, args string) (Result, error) {
			name := strings.TrimSpace(args)
			if name == "" {
				return Result{}, fmt.Errorf("context: missing argument")
			}
			cmd := exec.CommandContext(ctx, "kubectl", "config", "use-context", name)
			if out, err := cmd.CombinedOutput(); err != nil {
				return Result{}, fmt.Errorf("context: %w: %s", err, strings.TrimSpace(string(out)))
			}
			return Result{Message: "context → " + name}, nil
		},
	})

	r.Register(Command{
		Name:        "profile",
		Description: "switch aws profile",
		Handler: func(_ context.Context, args string) (Result, error) {
			p := strings.TrimSpace(args)
			if p == "" {
				return Result{}, fmt.Errorf("profile: missing argument")
			}
			// The actual awssession.SetProfile call is done by the calling
			// layer to keep this package free of cross-package dependencies.
			return Result{Message: "profile → " + p}, nil
		},
	})

	return r
}
