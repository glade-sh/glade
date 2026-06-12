package enterprise

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ServiceConfig struct {
	Version           int      `json:"version"`
	Mode              string   `json:"mode"`
	CalloutFixtures   []string `json:"calloutFixtures,omitempty"`
	AsyncDrain        bool     `json:"asyncDrain,omitempty"`
	AsyncMaxDepth     int      `json:"asyncMaxDepth,omitempty"`
	PlatformEventsOut string   `json:"platformEventsOut,omitempty"`
}

func LoadServiceConfig(path string) (ServiceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ServiceConfig{}, err
	}
	cfg, err := parseServiceConfigSubset(string(data))
	if err != nil {
		return ServiceConfig{}, err
	}
	if cfg.Version != 0 {
		return ServiceConfig{}, fmt.Errorf("unsupported services.yml version %d", cfg.Version)
	}
	base := filepath.Dir(path)
	for _, fixture := range cfg.CalloutFixtures {
		full := fixture
		if !filepath.IsAbs(full) {
			full = filepath.Join(base, filepath.FromSlash(fixture))
		}
		if _, err := os.Stat(full); err != nil {
			return ServiceConfig{}, fmt.Errorf("callout fixture %s: %w", fixture, err)
		}
	}
	if cfg.AsyncDrain && cfg.AsyncMaxDepth <= 0 {
		return ServiceConfig{}, fmt.Errorf("asyncMaxDepth must be positive when asyncDrain is true")
	}
	if cfg.Mode == "" {
		cfg.Mode = "strict"
	}
	if cfg.Mode != "strict" && cfg.Mode != "permissive" {
		return ServiceConfig{}, fmt.Errorf("mode must be strict or permissive")
	}
	return cfg, nil
}

func ValidateServiceConfig(path string) error {
	_, err := LoadServiceConfig(path)
	return err
}

func parseServiceConfigSubset(src string) (ServiceConfig, error) {
	var cfg ServiceConfig
	for lineNo, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(stripServiceComment(raw))
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return ServiceConfig{}, fmt.Errorf("services.yml:%d: expected key: value", lineNo+1)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "version":
			n, err := strconv.Atoi(trimServiceScalar(value))
			if err != nil {
				return ServiceConfig{}, fmt.Errorf("services.yml:%d: version must be an integer", lineNo+1)
			}
			cfg.Version = n
		case "mode":
			cfg.Mode = trimServiceScalar(value)
		case "calloutFixtures":
			values, err := parseServiceInlineList(value)
			if err != nil {
				return ServiceConfig{}, fmt.Errorf("services.yml:%d: %w", lineNo+1, err)
			}
			cfg.CalloutFixtures = values
		case "asyncDrain":
			b, err := strconv.ParseBool(trimServiceScalar(value))
			if err != nil {
				return ServiceConfig{}, fmt.Errorf("services.yml:%d: asyncDrain must be true or false", lineNo+1)
			}
			cfg.AsyncDrain = b
		case "asyncMaxDepth":
			n, err := strconv.Atoi(trimServiceScalar(value))
			if err != nil {
				return ServiceConfig{}, fmt.Errorf("services.yml:%d: asyncMaxDepth must be an integer", lineNo+1)
			}
			cfg.AsyncMaxDepth = n
		case "platformEventsOut":
			cfg.PlatformEventsOut = trimServiceScalar(value)
		default:
			return ServiceConfig{}, fmt.Errorf("services.yml:%d: unsupported service config key %q", lineNo+1, key)
		}
	}
	return cfg, nil
}

func stripServiceComment(s string) string {
	if idx := strings.IndexByte(s, '#'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func trimServiceScalar(s string) string {
	return strings.Trim(strings.TrimSpace(s), `"'`)
}

func parseServiceInlineList(s string) ([]string, error) {
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
		value := trimServiceScalar(part)
		if value == "" {
			return nil, fmt.Errorf("empty inline list value")
		}
		out = append(out, value)
	}
	return out, nil
}
