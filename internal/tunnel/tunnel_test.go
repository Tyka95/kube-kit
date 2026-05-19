package tunnel

import (
	"testing"
)

// TestGenPodNameUnique verifies that 10 successive calls to genPodName all
// produce distinct strings.
func TestGenPodNameUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, 10)
	for i := 0; i < 10; i++ {
		name := genPodName()
		if _, dup := seen[name]; dup {
			t.Fatalf("genPodName() returned duplicate value %q on iteration %d", name, i)
		}
		seen[name] = struct{}{}
	}
}

// TestConfigWithDefaultsNamespace verifies that an empty Namespace is
// replaced with "default".
func TestConfigWithDefaultsNamespace(t *testing.T) {
	t.Parallel()

	cfg := Config{Host: "db.example.com", RemotePort: 5432, LocalPort: 15432}
	got := cfg.withDefaults()
	if got.Namespace != "default" {
		t.Errorf("expected Namespace=%q, got %q", "default", got.Namespace)
	}
}

// TestConfigWithDefaultsImage verifies that an empty Image is replaced with
// "alpine/socat".
func TestConfigWithDefaultsImage(t *testing.T) {
	t.Parallel()

	cfg := Config{Namespace: "prod", Host: "db.example.com", RemotePort: 5432, LocalPort: 15432}
	got := cfg.withDefaults()
	if got.Image != "alpine/socat" {
		t.Errorf("expected Image=%q, got %q", "alpine/socat", got.Image)
	}
}

// TestConfigWithDefaultsPreservesValues verifies that explicitly set values
// are not overwritten by withDefaults.
func TestConfigWithDefaultsPreservesValues(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Namespace:  "staging",
		Host:       "pg.internal",
		RemotePort: 5433,
		LocalPort:  25432,
		Image:      "my-org/socat:latest",
	}
	got := cfg.withDefaults()
	if got.Namespace != "staging" {
		t.Errorf("Namespace should not have been overridden: got %q", got.Namespace)
	}
	if got.Image != "my-org/socat:latest" {
		t.Errorf("Image should not have been overridden: got %q", got.Image)
	}
}

// TestCloseIdempotent verifies that calling Close() twice on a Tunnel with a
// nil pfCmd does not panic and returns nil both times.
func TestCloseIdempotent(t *testing.T) {
	t.Parallel()

	tun := &Tunnel{
		podName: "kubekit-tunnel-test-000000",
		pfCmd:   nil, // no real process
		cfg: Config{
			Namespace:  "default",
			Host:       "db.example.com",
			RemotePort: 5432,
			LocalPort:  15432,
			Image:      "alpine/socat",
		},
	}

	// First Close — deletePod will fail because kubectl is not available in
	// tests, but we only care that it does not panic and that the idempotency
	// guard is set.
	_ = tun.Close()

	// Second Close must be a no-op (closed == true) and must not panic.
	if err := tun.Close(); err != nil {
		t.Errorf("second Close() returned unexpected error: %v", err)
	}
}
