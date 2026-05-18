package commands_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Tyka95/kube-kit/internal/commands"
)

// ── Empty registry ────────────────────────────────────────────────────────────

func TestEmptyRegistry_EmptyInput(t *testing.T) {
	r := commands.NewRegistry()
	_, err := r.Run(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestEmptyRegistry_UnknownCommand(t *testing.T) {
	r := commands.NewRegistry()
	_, err := r.Run(context.Background(), "foo")
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected error to contain 'unknown', got: %v", err)
	}
}

// ── Dispatch with whitespace-trimmed args ─────────────────────────────────────

func TestDispatch_WhitespaceTrimmedArgs(t *testing.T) {
	var capturedArgs string

	r := commands.NewRegistry()
	r.Register(commands.Command{
		Name:        "foo",
		Description: "test command",
		Handler: func(_ context.Context, args string) (commands.Result, error) {
			capturedArgs = args
			return commands.Result{Message: "ok"}, nil
		},
	})

	result, err := r.Run(context.Background(), ":foo  bar baz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedArgs != "bar baz" {
		t.Fatalf("expected args=%q, got %q", "bar baz", capturedArgs)
	}
	if result.Message != "ok" {
		t.Fatalf("expected message=%q, got %q", "ok", result.Message)
	}
}

// ── Builtins registration ─────────────────────────────────────────────────────

func TestBuiltins_ListLength(t *testing.T) {
	r := commands.Builtins()
	list := r.List()
	// q, quit, help, ns, context, profile → 6 entries
	if len(list) != 6 {
		t.Fatalf("expected 6 builtins, got %d", len(list))
	}
}

func TestBuiltins_Names(t *testing.T) {
	r := commands.Builtins()
	want := []string{"q", "quit", "help", "ns", "context", "profile"}
	list := r.List()

	nameSet := make(map[string]bool, len(list))
	for _, c := range list {
		nameSet[c.Name] = true
	}

	for _, name := range want {
		if !nameSet[name] {
			t.Errorf("expected builtin %q to be registered", name)
		}
	}
}

// ── :q and :quit → Sentinel "quit" ───────────────────────────────────────────

func TestQ_Sentinel(t *testing.T) {
	r := commands.Builtins()
	result, err := r.Run(context.Background(), ":q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Sentinel != "quit" {
		t.Fatalf("expected Sentinel=%q, got %q", "quit", result.Sentinel)
	}
}

func TestQuit_Sentinel(t *testing.T) {
	r := commands.Builtins()
	result, err := r.Run(context.Background(), ":quit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Sentinel != "quit" {
		t.Fatalf("expected Sentinel=%q, got %q", "quit", result.Sentinel)
	}
}

// ── :help → Sentinel "help" ───────────────────────────────────────────────────

func TestHelp_Sentinel(t *testing.T) {
	r := commands.Builtins()
	result, err := r.Run(context.Background(), ":help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Sentinel != "help" {
		t.Fatalf("expected Sentinel=%q, got %q", "help", result.Sentinel)
	}
}

// ── Duplicate registration preserves slot ─────────────────────────────────────

func TestRegister_DuplicatePreservesOrder(t *testing.T) {
	r := commands.NewRegistry()
	r.Register(commands.Command{Name: "a", Handler: func(_ context.Context, _ string) (commands.Result, error) {
		return commands.Result{Message: "a-v1"}, nil
	}})
	r.Register(commands.Command{Name: "b", Handler: func(_ context.Context, _ string) (commands.Result, error) {
		return commands.Result{Message: "b"}, nil
	}})
	// Overwrite "a" — should stay in first slot.
	r.Register(commands.Command{Name: "a", Handler: func(_ context.Context, _ string) (commands.Result, error) {
		return commands.Result{Message: "a-v2"}, nil
	}})

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(list))
	}
	if list[0].Name != "a" {
		t.Fatalf("expected first command to be 'a', got %q", list[0].Name)
	}

	result, err := r.Run(context.Background(), "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Message != "a-v2" {
		t.Fatalf("expected updated handler to run, got message=%q", result.Message)
	}
}

// ── Handler error propagates ──────────────────────────────────────────────────

func TestRun_HandlerError(t *testing.T) {
	r := commands.NewRegistry()
	r.Register(commands.Command{
		Name: "fail",
		Handler: func(_ context.Context, _ string) (commands.Result, error) {
			return commands.Result{}, errors.New("boom")
		},
	})

	_, err := r.Run(context.Background(), "fail")
	if err == nil {
		t.Fatal("expected error from handler, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to contain 'boom', got: %v", err)
	}
}

// ── :profile → Message (no kubectl side effects) ──────────────────────────────

func TestProfile_Message(t *testing.T) {
	r := commands.Builtins()
	result, err := r.Run(context.Background(), ":profile staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Message != "profile → staging" {
		t.Fatalf("expected message=%q, got %q", "profile → staging", result.Message)
	}
	if result.Sentinel != "" {
		t.Fatalf("expected empty Sentinel, got %q", result.Sentinel)
	}
}
