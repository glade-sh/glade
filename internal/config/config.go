package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrNotFound = errors.New("oaer config not found")

type Config struct {
	Project ProjectConfig `json:"project"`
	Org     OrgConfig     `json:"org"`
}

type ProjectConfig struct {
	Root             string   `json:"root"`
	PackageDirs      []string `json:"packageDirs"`
	DefaultNamespace string   `json:"defaultNamespace"`
}

type OrgConfig struct {
	Features []string `json:"features"`
}

func LoadNearest(start string) (Config, string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return Config{}, "", err
	}

	for {
		path := filepath.Join(dir, "oaer.yml")
		if _, err := os.Stat(path); err == nil {
			cfg, err := LoadFile(path)
			return cfg, path, err
		} else if !errors.Is(err, os.ErrNotExist) {
			return Config{}, "", err
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return Config{}, "", ErrNotFound
		}
		dir = parent
	}
}

func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return parseYAMLSubset(string(data))
}

func parseYAMLSubset(src string) (Config, error) {
	var cfg Config
	var section string

	for lineNo, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		if !strings.HasPrefix(raw, " ") && strings.HasSuffix(line, ":") {
			section = strings.TrimSuffix(line, ":")
			continue
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return Config{}, fmt.Errorf("oaer.yml:%d: expected key: value", lineNo+1)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch section + "." + key {
		case "project.root":
			cfg.Project.Root = trimScalar(value)
		case "project.defaultNamespace":
			cfg.Project.DefaultNamespace = trimScalar(value)
		case "project.packageDirs":
			values, err := parseInlineList(value)
			if err != nil {
				return Config{}, fmt.Errorf("oaer.yml:%d: %w", lineNo+1, err)
			}
			cfg.Project.PackageDirs = values
		case "org.features":
			values, err := parseInlineList(value)
			if err != nil {
				return Config{}, fmt.Errorf("oaer.yml:%d: %w", lineNo+1, err)
			}
			cfg.Org.Features = values
		default:
			return Config{}, fmt.Errorf("oaer.yml:%d: unsupported config key %q", lineNo+1, key)
		}
	}

	return cfg, nil
}

func stripComment(s string) string {
	if idx := strings.IndexByte(s, '#'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func trimScalar(s string) string {
	return strings.Trim(strings.TrimSpace(s), `"'`)
}

func parseInlineList(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("expected inline list")
	}
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, "["), "]"))
	if s == "" {
		return nil, nil
	}

	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, trimScalar(part))
	}
	return out, nil
}
