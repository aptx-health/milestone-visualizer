// Package config loads .msv.yaml so agents don't have to repeat flags.
//
// Search order:
//   1. --config <path>
//   2. $MSV_CONFIG
//   3. ./.msv.yaml (walk up from CWD to git root)
//   4. defaults (empty)
//
// Flag values always win over config values.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Owner     string `yaml:"owner"`
	Repo      string `yaml:"repo"`
	Milestone string `yaml:"milestone"`
	GraphFile string `yaml:"graph_file"`
	// FailOn declares which doctor findings should produce a non-zero exit.
	// Values: "mismatch", "orphans", "cycles", "unknown-graph-node", "any"
	FailOn []string `yaml:"fail_on,omitempty"`
}

// OwnerRepo returns "owner/repo" or "" if either half is missing.
func (c Config) OwnerRepo() string {
	if c.Owner == "" || c.Repo == "" {
		return ""
	}
	return c.Owner + "/" + c.Repo
}

// Load reads config from the resolved path. Returns an empty Config with
// no error when no file is found and explicit=="" (implicit search).
func Load(explicit string) (Config, string, error) {
	path := explicit
	if path == "" {
		if p := os.Getenv("MSV_CONFIG"); p != "" {
			path = p
		}
	}
	if path == "" {
		p, err := findUp(".msv.yaml")
		if err == nil {
			path = p
		}
	}
	if path == "" {
		return Config{}, "", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, path, fmt.Errorf("read %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Config{}, path, fmt.Errorf("parse %s: %w", path, err)
	}
	// Normalize
	c.Owner = strings.TrimSpace(c.Owner)
	c.Repo = strings.TrimSpace(c.Repo)
	c.Milestone = strings.TrimSpace(c.Milestone)
	c.GraphFile = strings.TrimSpace(c.GraphFile)
	return c, path, nil
}

// findUp walks upward from CWD looking for `name`, stopping at $HOME or /.
func findUp(name string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	home, _ := os.UserHomeDir()
	dir := cwd
	for {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir || dir == home || parent == "/" {
			return "", fmt.Errorf("no %s found upward from %s", name, cwd)
		}
		dir = parent
	}
}

// Merge overlays flag values onto config values. Flag empty strings are
// treated as "not provided" and preserve the config value.
type Overrides struct {
	OwnerRepo string
	Milestone string
	GraphFile string
}

func (c Config) Merge(o Overrides) Config {
	if o.OwnerRepo != "" {
		parts := strings.SplitN(o.OwnerRepo, "/", 2)
		if len(parts) == 2 {
			c.Owner, c.Repo = parts[0], parts[1]
		}
	}
	if o.Milestone != "" {
		c.Milestone = o.Milestone
	}
	if o.GraphFile != "" {
		c.GraphFile = o.GraphFile
	}
	return c
}
