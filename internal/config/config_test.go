package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Tyka95/kube-kit/internal/config"
)

// TestLoadFromPath_NonExistent verifies that a missing file returns an empty
// Config and no error.
func TestLoadFromPath_NonExistent(t *testing.T) {
	cfg, err := config.LoadFromPath("/does/not/exist/kubekit/config")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil Config")
	}
	if len(cfg.AWSRegions) != 0 || cfg.DefaultNamespace != "" ||
		cfg.DefaultProfile != "" || len(cfg.DBTargets) != 0 {
		t.Fatalf("expected empty Config, got %+v", cfg)
	}
}

// TestLoadFromPath_AllKeys verifies all four key types are parsed correctly.
func TestLoadFromPath_AllKeys(t *testing.T) {
	content := `# This is a comment

aws_regions=us-east-1, eu-west-1 ,  ap-south-1
default_namespace=production
default_profile=my-sso-profile
db_target=Primary DB|db.example.com|5432
db_target=Replica|replica.example.com|5433
`
	path := writeTemp(t, content)

	cfg, err := config.LoadFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// aws_regions
	want := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	if len(cfg.AWSRegions) != len(want) {
		t.Fatalf("aws_regions: want %v, got %v", want, cfg.AWSRegions)
	}
	for i, r := range want {
		if cfg.AWSRegions[i] != r {
			t.Errorf("aws_regions[%d]: want %q, got %q", i, r, cfg.AWSRegions[i])
		}
	}

	// default_namespace
	if cfg.DefaultNamespace != "production" {
		t.Errorf("default_namespace: want %q, got %q", "production", cfg.DefaultNamespace)
	}

	// default_profile
	if cfg.DefaultProfile != "my-sso-profile" {
		t.Errorf("default_profile: want %q, got %q", "my-sso-profile", cfg.DefaultProfile)
	}

	// db_targets
	if len(cfg.DBTargets) != 2 {
		t.Fatalf("db_targets: want 2 entries, got %d", len(cfg.DBTargets))
	}
	assertDBTarget(t, cfg.DBTargets[0], "Primary DB", "db.example.com", 5432)
	assertDBTarget(t, cfg.DBTargets[1], "Replica", "replica.example.com", 5433)
}

// TestLoadFromPath_CommentsAndBlanks verifies that comments and blank lines do
// not cause errors.
func TestLoadFromPath_CommentsAndBlanks(t *testing.T) {
	content := `# comment at top

   # indented comment
default_namespace=staging

`
	path := writeTemp(t, content)

	cfg, err := config.LoadFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultNamespace != "staging" {
		t.Errorf("default_namespace: want %q, got %q", "staging", cfg.DefaultNamespace)
	}
}

// TestLoadFromPath_TwoDBTargets checks that two db_target lines both appear in
// order.
func TestLoadFromPath_TwoDBTargets(t *testing.T) {
	content := `db_target=Alpha|alpha.db|5432
db_target=Beta|beta.db|5433
`
	path := writeTemp(t, content)

	cfg, err := config.LoadFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.DBTargets) != 2 {
		t.Fatalf("want 2 db_targets, got %d", len(cfg.DBTargets))
	}
	assertDBTarget(t, cfg.DBTargets[0], "Alpha", "alpha.db", 5432)
	assertDBTarget(t, cfg.DBTargets[1], "Beta", "beta.db", 5433)
}

// TestLoadFromPath_AWSRegionsWhitespace verifies that whitespace around commas
// is trimmed correctly.
func TestLoadFromPath_AWSRegionsWhitespace(t *testing.T) {
	content := "aws_regions=us-east-1, eu-west-1 ,  ap-south-1\n"
	path := writeTemp(t, content)

	cfg, err := config.LoadFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	if len(cfg.AWSRegions) != len(want) {
		t.Fatalf("aws_regions: want %v, got %v", want, cfg.AWSRegions)
	}
	for i, r := range want {
		if cfg.AWSRegions[i] != r {
			t.Errorf("aws_regions[%d]: want %q, got %q", i, r, cfg.AWSRegions[i])
		}
	}
}

// TestLoadFromPath_DBTargetMissingPort checks that a missing port defaults to
// 5432.
func TestLoadFromPath_DBTargetMissingPort(t *testing.T) {
	content := "db_target=Bad Target|host.example.com\n"
	path := writeTemp(t, content)

	cfg, err := config.LoadFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.DBTargets) != 1 {
		t.Fatalf("want 1 db_target, got %d", len(cfg.DBTargets))
	}
	assertDBTarget(t, cfg.DBTargets[0], "Bad Target", "host.example.com", 5432)
}

// helpers

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeTemp: %v", err)
	}
	return path
}

func assertDBTarget(t *testing.T, dt config.DBTarget, name, host string, port int) {
	t.Helper()
	if dt.Name != name {
		t.Errorf("DBTarget.Name: want %q, got %q", name, dt.Name)
	}
	if dt.Host != host {
		t.Errorf("DBTarget.Host: want %q, got %q", host, dt.Host)
	}
	if dt.Port != port {
		t.Errorf("DBTarget.Port: want %d, got %d", port, dt.Port)
	}
}
