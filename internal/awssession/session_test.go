package awssession_test

import (
	"context"
	"testing"

	"github.com/Tyka95/kube-kit/internal/awssession"
)

// TestNew_InitialStatus asserts that a freshly created Session reports
// StatusUnknown before any work is done.
func TestNew_InitialStatus(t *testing.T) {
	s := awssession.New()
	snap := s.Snapshot()
	if snap.Status != awssession.StatusUnknown {
		t.Fatalf("expected StatusUnknown, got %q", snap.Status)
	}
}

// TestResolve_DoesNotPanic ensures Resolve completes without panicking even
// when kubectl and aws are missing (or context timeouts, etc.).
func TestResolve_DoesNotPanic(t *testing.T) {
	s := awssession.New()
	ctx := context.Background()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Resolve panicked: %v", r)
		}
	}()

	id := s.Resolve(ctx)

	// Status is still unknown after Resolve (it does not call AWS).
	if id.Status != awssession.StatusUnknown {
		t.Fatalf("expected StatusUnknown after Resolve, got %q", id.Status)
	}
}
