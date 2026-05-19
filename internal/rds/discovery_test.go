package rds

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ─── cache state helper ───────────────────────────────────────────────────────

func cacheIsEmpty(d *Discoverer) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cache == nil
}

// ─── New() starts with empty cache ───────────────────────────────────────────

func TestNew_EmptyCache(t *testing.T) {
	d := New()
	if !cacheIsEmpty(d) {
		t.Fatal("expected empty cache on New()")
	}
}

// ─── Discover with missing aws binary returns error, cache stays empty ────────

func TestDiscover_MissingAWS_ReturnsError(t *testing.T) {
	// Point PATH at an empty temp dir so `aws` cannot be found.
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	d := New()
	endpoints, err := d.Discover(context.Background(), "testprofile", "us-east-1", "123456789")
	if err == nil {
		t.Fatal("expected an error when aws binary is missing, got nil")
	}
	if len(endpoints) != 0 {
		t.Fatalf("expected empty endpoint slice on error, got %d endpoints", len(endpoints))
	}
	if !cacheIsEmpty(d) {
		t.Fatal("cache should remain empty after a failed discover")
	}
}

// ─── Invalidate clears the cache ─────────────────────────────────────────────

func TestInvalidate_ClearsCache(t *testing.T) {
	d := New()
	// Manually populate the cache to simulate a prior successful discover.
	d.mu.Lock()
	d.cache = &cacheEntry{
		account:   "123456789",
		region:    "us-east-1",
		timestamp: time.Now(),
		endpoints: []Endpoint{{Identifier: "test", Host: "test.rds.amazonaws.com", Port: 5432}},
	}
	d.mu.Unlock()

	if cacheIsEmpty(d) {
		t.Fatal("pre-condition: cache should be non-empty before Invalidate")
	}

	d.Invalidate()

	if !cacheIsEmpty(d) {
		t.Fatal("cache should be empty after Invalidate()")
	}
}

// ─── Invalidate triggers fresh discover on next call ─────────────────────────

func TestInvalidate_ThenDiscover_AttemptsRefresh(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	d := New()
	// Populate the cache.
	d.mu.Lock()
	d.cache = &cacheEntry{
		account:   "123456789",
		region:    "us-east-1",
		timestamp: time.Now(),
		endpoints: []Endpoint{{Identifier: "test", Host: "test.rds.amazonaws.com", Port: 5432}},
	}
	d.mu.Unlock()

	d.Invalidate()

	// Next Discover must attempt a fresh AWS call; since `aws` is missing it errors.
	_, err := d.Discover(context.Background(), "testprofile", "us-east-1", "123456789")
	if err == nil {
		t.Fatal("expected error after Invalidate() + Discover() with no aws binary")
	}
}

// ─── Cache hit: same account+region within TTL ────────────────────────────────

func TestDiscover_CacheHit(t *testing.T) {
	d := New()
	expected := []Endpoint{
		{Identifier: "prod-cluster", Host: "prod.cluster.us-east-1.rds.amazonaws.com", Port: 5432, Region: "us-east-1", Profile: "prod", Kind: KindAuroraCluster},
	}
	d.mu.Lock()
	d.cache = &cacheEntry{
		account:   "111222333",
		region:    "us-east-1",
		timestamp: time.Now(),
		endpoints: expected,
	}
	d.mu.Unlock()

	// With PATH pointing at empty dir, any real aws call would fail.
	// A cache hit should return immediately without calling aws.
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	got, err := d.Discover(context.Background(), "prod", "us-east-1", "111222333")
	if err != nil {
		t.Fatalf("unexpected error on cache hit: %v", err)
	}
	if len(got) != len(expected) {
		t.Fatalf("expected %d endpoints, got %d", len(expected), len(got))
	}
	if got[0].Identifier != expected[0].Identifier {
		t.Errorf("expected identifier %q, got %q", expected[0].Identifier, got[0].Identifier)
	}
}

// ─── parseClusters unit tests ─────────────────────────────────────────────────

func TestParseClusters_Basic(t *testing.T) {
	input := []byte("prod-aurora\tprod.cluster.us-east-1.rds.amazonaws.com\t5432\n" +
		"staging-aurora\tstaging.cluster.us-east-1.rds.amazonaws.com\t3306\n")

	got := parseClusters(input, "us-east-1", "myprofile")

	if len(got) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(got))
	}

	if got[0].Identifier != "prod-aurora" {
		t.Errorf("expected identifier 'prod-aurora', got %q", got[0].Identifier)
	}
	if got[0].Host != "prod.cluster.us-east-1.rds.amazonaws.com" {
		t.Errorf("unexpected host: %q", got[0].Host)
	}
	if got[0].Port != 5432 {
		t.Errorf("expected port 5432, got %d", got[0].Port)
	}
	if got[0].Kind != KindAuroraCluster {
		t.Errorf("expected kind %q, got %q", KindAuroraCluster, got[0].Kind)
	}
	if got[0].Region != "us-east-1" {
		t.Errorf("expected region 'us-east-1', got %q", got[0].Region)
	}
	if got[0].Profile != "myprofile" {
		t.Errorf("expected profile 'myprofile', got %q", got[0].Profile)
	}
	if got[1].Port != 3306 {
		t.Errorf("expected port 3306, got %d", got[1].Port)
	}
}

func TestParseClusters_DefaultPort(t *testing.T) {
	// Missing port column → default 5432.
	input := []byte("my-cluster\tmy-cluster.us-west-2.rds.amazonaws.com\n")
	got := parseClusters(input, "us-west-2", "default")
	if len(got) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(got))
	}
	if got[0].Port != 5432 {
		t.Errorf("expected default port 5432, got %d", got[0].Port)
	}
}

func TestParseClusters_SkipsNoneEndpoint(t *testing.T) {
	input := []byte("bad-cluster\tNone\t5432\n" +
		"good-cluster\tgood.us-east-1.rds.amazonaws.com\t5432\n")
	got := parseClusters(input, "us-east-1", "default")
	if len(got) != 1 {
		t.Fatalf("expected 1 endpoint (None skipped), got %d", len(got))
	}
	if got[0].Identifier != "good-cluster" {
		t.Errorf("unexpected identifier: %q", got[0].Identifier)
	}
}

func TestParseClusters_SkipsBlankLines(t *testing.T) {
	input := []byte("\n\nmy-cluster\tmy.rds.amazonaws.com\t5432\n\n")
	got := parseClusters(input, "us-east-1", "default")
	if len(got) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(got))
	}
}

func TestParseClusters_Empty(t *testing.T) {
	got := parseClusters([]byte(""), "us-east-1", "default")
	if len(got) != 0 {
		t.Fatalf("expected 0 endpoints from empty input, got %d", len(got))
	}
}

// ─── parseInstances unit tests ────────────────────────────────────────────────

func TestParseInstances_Basic(t *testing.T) {
	input := []byte("prod-instance\tprod.instance.us-east-1.rds.amazonaws.com\t5432\n")
	got := parseInstances(input, "us-east-1", "prod")
	if len(got) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(got))
	}
	if got[0].Kind != KindRDSInstance {
		t.Errorf("expected kind %q, got %q", KindRDSInstance, got[0].Kind)
	}
	if got[0].Identifier != "prod-instance" {
		t.Errorf("unexpected identifier: %q", got[0].Identifier)
	}
}

func TestParseInstances_SkipsNone(t *testing.T) {
	input := []byte("broken\tNone\t5432\n")
	got := parseInstances(input, "us-east-1", "default")
	if len(got) != 0 {
		t.Fatalf("expected 0 endpoints, got %d", len(got))
	}
}

// ─── Discover with fake aws script returning valid output ────────────────────

func TestDiscover_FakeAWSSuccess(t *testing.T) {
	// Write a tiny shell script that acts as `aws`.
	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "aws")

	// The script distinguishes clusters vs instances by checking its arguments.
	script := `#!/bin/sh
case "$*" in
  *describe-db-clusters*)
    printf "prod-aurora\tprod.cluster.us-east-1.rds.amazonaws.com\t5432\n"
    ;;
  *describe-db-instances*)
    printf "prod-instance\tprod.instance.us-east-1.rds.amazonaws.com\t5432\n"
    ;;
esac
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake aws script: %v", err)
	}
	t.Setenv("PATH", scriptDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	d := New()
	endpoints, err := d.Discover(context.Background(), "testprofile", "us-east-1", "123456789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}
	if cacheIsEmpty(d) {
		t.Fatal("cache should be populated after successful discover")
	}

	// Second call should be a cache hit (no exec needed).
	endpoints2, err := d.Discover(context.Background(), "testprofile", "us-east-1", "123456789")
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if len(endpoints2) != 2 {
		t.Fatalf("expected 2 endpoints on cache hit, got %d", len(endpoints2))
	}
}

// ─── Cache miss on different region ──────────────────────────────────────────

func TestDiscover_CacheMissOnRegionChange(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	d := New()
	// Pre-populate cache for us-east-1.
	d.mu.Lock()
	d.cache = &cacheEntry{
		account:   "111222333",
		region:    "us-east-1",
		timestamp: time.Now(),
		endpoints: []Endpoint{{Identifier: "cached", Host: "cached.rds.amazonaws.com", Port: 5432}},
	}
	d.mu.Unlock()

	// Discover for us-west-2 should be a cache miss → tries aws → fails.
	_, err := d.Discover(context.Background(), "myprofile", "us-west-2", "111222333")
	if err == nil {
		t.Fatal("expected error on cache miss with no aws binary")
	}
}
