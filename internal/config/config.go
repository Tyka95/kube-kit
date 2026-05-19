package config

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DBTarget represents a database tunnel target parsed from a db_target= line.
type DBTarget struct {
	Name string
	Host string
	Port int
}

// Config holds the parsed contents of the kubekit config file.
type Config struct {
	AWSRegions       []string
	DefaultNamespace string
	DefaultProfile   string
	DBTargets        []DBTarget
}

// Load reads ~/.config/kubekit/config and returns the parsed Config.
// If the file does not exist it returns an empty Config and a nil error.
// An error is returned only when the file exists but cannot be read or parsed.
func Load() (*Config, error) {
	path := filepath.Join(os.Getenv("HOME"), ".config", "kubekit", "config")
	return LoadFromPath(path)
}

// LoadFromPath is identical to Load but reads from an explicit path.
// This is useful in tests.
func LoadFromPath(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}
	defer f.Close()

	return parse(f)
}

// parse reads key=value lines from r and populates a Config.
func parse(r *os.File) (*Config, error) {
	cfg := &Config{}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments (leading whitespace then #).
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}

		// Split on the first '=' only.
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		switch key {
		case "aws_regions":
			parts := strings.Split(val, ",")
			cfg.AWSRegions = make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					cfg.AWSRegions = append(cfg.AWSRegions, p)
				}
			}

		case "default_namespace":
			cfg.DefaultNamespace = val

		case "default_profile":
			cfg.DefaultProfile = val

		case "db_target":
			cfg.DBTargets = append(cfg.DBTargets, parseDBTarget(val))
		}
		// Unknown keys are silently ignored.
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// parseDBTarget parses a "Name|host|port" value.
// Port defaults to 5432 when missing or unparseable.
func parseDBTarget(val string) DBTarget {
	const defaultPort = 5432

	parts := strings.SplitN(val, "|", 3)

	dt := DBTarget{Port: defaultPort}

	if len(parts) >= 1 {
		dt.Name = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		dt.Host = strings.TrimSpace(parts[1])
	}
	if len(parts) >= 3 {
		p, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err == nil {
			dt.Port = p
		}
	}

	return dt
}
