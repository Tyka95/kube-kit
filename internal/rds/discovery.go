package rds

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const cacheTTL = 60 * time.Second

// Kind identifies whether an endpoint is an Aurora cluster or a standalone RDS instance.
type Kind string

const (
	KindAuroraCluster Kind = "aurora-cluster"
	KindRDSInstance   Kind = "rds-instance"
)

// Endpoint represents a single discoverable RDS / Aurora endpoint.
type Endpoint struct {
	Identifier string // e.g. "production-rds-aurora-cluster"
	Host       string // FQDN
	Port       int
	Region     string
	Profile    string // the profile this endpoint was discovered under (for the meta column)
	Kind       Kind
}

type cacheEntry struct {
	account   string
	region    string
	timestamp time.Time
	endpoints []Endpoint
}

// Discoverer holds an in-memory cache keyed on (account, region) with a 60s TTL.
type Discoverer struct {
	mu    sync.Mutex
	cache *cacheEntry
}

// New returns a new Discoverer with an empty cache.
func New() *Discoverer {
	return &Discoverer{}
}

// Discover runs `aws rds describe-db-clusters` and `aws rds describe-db-instances`
// for the given profile/region. Returns cached results on cache hit (same account+region,
// timestamp within 60s). On cache miss, shells out to the aws CLI and populates the cache.
//
// If either aws command fails, the error is returned and the cache timestamp is NOT updated
// so the next call will retry immediately.
func (d *Discoverer) Discover(ctx context.Context, profile, region, account string) ([]Endpoint, error) {
	if region == "" {
		return nil, fmt.Errorf("aws region unset; configure a kubectl EKS context, $AWS_REGION, or aws_regions= in ~/.config/kubekit/config")
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	// Cache hit: same account+region, within TTL.
	if d.cache != nil &&
		d.cache.account == account &&
		d.cache.region == region &&
		time.Since(d.cache.timestamp) < cacheTTL {
		return d.cache.endpoints, nil
	}

	// Build shared aws CLI args.
	args := buildAWSArgs(profile, region)

	// Query Aurora clusters.
	clusterArgs := append(args,
		"rds", "describe-db-clusters",
		"--query", "DBClusters[].[DBClusterIdentifier,Endpoint,Port]",
	)
	clusterOut, err := runAWS(ctx, clusterArgs)
	if err != nil {
		return []Endpoint{}, err
	}

	// Query standalone RDS instances (those not part of a cluster).
	// The JMESPath literal null must be backtick-quoted in the AWS CLI query syntax.
	instanceArgs := append(args,
		"rds", "describe-db-instances",
		"--query", "DBInstances[?DBClusterIdentifier==`null`].[DBInstanceIdentifier,Endpoint.Address,Endpoint.Port]",
	)
	instanceOut, err := runAWS(ctx, instanceArgs)
	if err != nil {
		return []Endpoint{}, err
	}

	// Both queries succeeded — parse and populate the cache.
	endpoints := parseClusters(clusterOut, region, profile)
	endpoints = append(endpoints, parseInstances(instanceOut, region, profile)...)

	// Initialise or update cache.
	if d.cache == nil {
		d.cache = &cacheEntry{}
	}
	d.cache.account = account
	d.cache.region = region
	d.cache.timestamp = time.Now()
	d.cache.endpoints = endpoints

	return endpoints, nil
}

// Invalidate clears the cache. Call this on auth state changes (e.g. profile switch).
func (d *Discoverer) Invalidate() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache = nil
}

// buildAWSArgs constructs the common aws CLI argument slice.
func buildAWSArgs(profile, region string) []string {
	args := []string{"aws"}
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	args = append(args,
		"--region", region,
		"--output", "text",
		"--cli-connect-timeout", "3",
		"--cli-read-timeout", "8",
	)
	return args
}

// runAWS executes the given command (first element is the binary name), captures stdout,
// and returns stderr's first line as the error message if the command fails.
func runAWS(ctx context.Context, args []string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no command provided")
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		firstLine := firstNonEmptyLine(stderr.Bytes())
		if firstLine == "" {
			firstLine = err.Error()
		}
		return nil, fmt.Errorf("%s", firstLine)
	}
	return stdout.Bytes(), nil
}

// parseClusters parses `--output text` tab-separated output of describe-db-clusters.
// Expected columns: [DBClusterIdentifier, Endpoint, Port]
func parseClusters(data []byte, region, profile string) []Endpoint {
	return parseTabOutput(data, region, profile, KindAuroraCluster)
}

// parseInstances parses `--output text` tab-separated output of describe-db-instances.
// Expected columns: [DBInstanceIdentifier, Endpoint.Address, Endpoint.Port]
func parseInstances(data []byte, region, profile string) []Endpoint {
	return parseTabOutput(data, region, profile, KindRDSInstance)
}

// parseTabOutput is the shared tab-separated parser for both cluster and instance output.
func parseTabOutput(data []byte, region, profile string, kind Kind) []Endpoint {
	var endpoints []Endpoint
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, "\r")
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		id := strings.TrimSpace(parts[0])
		host := strings.TrimSpace(parts[1])
		if id == "" || host == "" || host == "None" {
			continue
		}

		port := 5432
		if len(parts) >= 3 {
			if p, err := strconv.Atoi(strings.TrimSpace(parts[2])); err == nil && p > 0 {
				port = p
			}
		}

		endpoints = append(endpoints, Endpoint{
			Identifier: id,
			Host:       host,
			Port:       port,
			Region:     region,
			Profile:    profile,
			Kind:       kind,
		})
	}
	return endpoints
}

// firstNonEmptyLine returns the first non-blank line from b.
func firstNonEmptyLine(b []byte) string {
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
