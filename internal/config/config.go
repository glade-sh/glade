package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/namespaceremap"
)

var ErrNotFound = errors.New("glade config not found")

type Config struct {
	Project ProjectConfig `json:"project"`
	Org     OrgConfig     `json:"org"`
}

type ProjectConfig struct {
	Root                       string                     `json:"root"`
	PackageDirs                []string                   `json:"packageDirs"`
	DefaultNamespace           string                     `json:"defaultNamespace"`
	NamespaceRemaps            []namespaceremap.Rule      `json:"namespaceRemaps,omitempty"`
	ManagedPackageDependencies []ManagedPackageDependency `json:"managedPackageDependencies,omitempty"`
	PackageShims               []PackageShim              `json:"packageShims,omitempty"`
}

type ManagedPackageDependency struct {
	Namespace    string `json:"namespace"`
	SourceRoot   string `json:"sourceRoot,omitempty"`
	ArtifactPath string `json:"artifactPath,omitempty"`
	Version      string `json:"version,omitempty"`
}

type PackageShim struct {
	Namespace  string `json:"namespace"`
	SourceRoot string `json:"sourceRoot"`
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
		path := filepath.Join(dir, "glade.yml")
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
	cfg, err := parseYAMLSubset(string(data))
	if err != nil {
		return Config{}, err
	}
	resolveManagedPackageDependencyPaths(&cfg, filepath.Dir(path))
	return cfg, nil
}

func parseYAMLSubset(src string) (Config, error) {
	var cfg Config
	var section string

	lines := strings.Split(src, "\n")
	for lineNo := 0; lineNo < len(lines); lineNo++ {
		raw := lines[lineNo]
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
			return Config{}, fmt.Errorf("glade.yml:%d: expected key: value", lineNo+1)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch section + "." + key {
		case "project.root":
			cfg.Project.Root = trimScalar(value)
		case "project.defaultNamespace":
			cfg.Project.DefaultNamespace = trimScalar(value)
		case "project.namespaceRemaps":
			values, err := parseInlineList(value)
			if err != nil {
				return Config{}, fmt.Errorf("glade.yml:%d: %w", lineNo+1, err)
			}
			remaps, err := parseNamespaceRemaps(values)
			if err != nil {
				return Config{}, fmt.Errorf("glade.yml:%d: %w", lineNo+1, err)
			}
			cfg.Project.NamespaceRemaps = remaps
		case "project.packageDirs":
			values, err := parseInlineList(value)
			if err != nil {
				return Config{}, fmt.Errorf("glade.yml:%d: %w", lineNo+1, err)
			}
			cfg.Project.PackageDirs = values
		case "project.managedPackageDependencies":
			if value == "" {
				entries, nextLine, err := parseManagedPackageDependencyBlock(lines, lineNo+1, leadingSpaces(raw))
				if err != nil {
					return Config{}, err
				}
				deps, err := parseManagedPackageDependencyEntries(entries)
				if err != nil {
					return Config{}, fmt.Errorf("glade.yml:%d: %w", lineNo+1, err)
				}
				cfg.Project.ManagedPackageDependencies = deps
				lineNo = nextLine - 1
			} else {
				values, err := parseInlineList(value)
				if err != nil {
					return Config{}, fmt.Errorf("glade.yml:%d: %w", lineNo+1, err)
				}
				deps, err := parseManagedPackageDependencies(values)
				if err != nil {
					return Config{}, fmt.Errorf("glade.yml:%d: %w", lineNo+1, err)
				}
				cfg.Project.ManagedPackageDependencies = deps
			}
		case "project.packageShims":
			values, err := parseInlineList(value)
			if err != nil {
				return Config{}, fmt.Errorf("glade.yml:%d: %w", lineNo+1, err)
			}
			shims, err := parsePackageShims(values)
			if err != nil {
				return Config{}, fmt.Errorf("glade.yml:%d: %w", lineNo+1, err)
			}
			cfg.Project.PackageShims = shims
		case "org.features":
			values, err := parseInlineList(value)
			if err != nil {
				return Config{}, fmt.Errorf("glade.yml:%d: %w", lineNo+1, err)
			}
			cfg.Org.Features = values
		default:
			return Config{}, fmt.Errorf("glade.yml:%d: unsupported config key %q", lineNo+1, key)
		}
	}

	return cfg, nil
}

func parseManagedPackageDependencyBlock(lines []string, start, parentIndent int) ([]ManagedPackageDependency, int, error) {
	var entries []ManagedPackageDependency
	current := -1
	for lineNo := start; lineNo < len(lines); lineNo++ {
		raw := lines[lineNo]
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		if leadingSpaces(raw) <= parentIndent {
			return entries, lineNo, nil
		}
		if strings.HasPrefix(line, "- ") {
			entries = append(entries, ManagedPackageDependency{})
			current = len(entries) - 1
			line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
			if line == "" {
				continue
			}
		} else if current < 0 {
			return nil, lineNo, fmt.Errorf("glade.yml:%d: expected managed package dependency list item", lineNo+1)
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, lineNo, fmt.Errorf("glade.yml:%d: expected key: value", lineNo+1)
		}
		key = strings.TrimSpace(key)
		value = trimScalar(value)
		switch key {
		case "namespace":
			entries[current].Namespace = value
		case "sourceRoot":
			entries[current].SourceRoot = value
		case "artifactPath":
			entries[current].ArtifactPath = value
		case "version":
			entries[current].Version = value
		default:
			return nil, lineNo, fmt.Errorf("glade.yml:%d: unsupported managed package dependency key %q", lineNo+1, key)
		}
	}
	return entries, len(lines), nil
}

func leadingSpaces(s string) int {
	return len(s) - len(strings.TrimLeft(s, " "))
}

func parseNamespaceRemaps(values []string) ([]namespaceremap.Rule, error) {
	seen := make(map[string]bool)
	rules := make([]namespaceremap.Rule, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if strings.Count(value, ":") != 1 {
			return nil, fmt.Errorf("invalid namespace remap %q: expected source:runtime", value)
		}
		from, to, _ := strings.Cut(value, ":")
		from = strings.TrimSpace(from)
		to = strings.TrimSpace(to)
		if from == "" || to == "" {
			return nil, fmt.Errorf("invalid namespace remap %q: source and runtime namespaces are required", value)
		}
		if strings.EqualFold(from, to) {
			return nil, fmt.Errorf("invalid namespace remap %q: source and runtime namespaces must differ", value)
		}
		fromKey := strings.ToLower(from)
		if seen[fromKey] {
			return nil, fmt.Errorf("duplicate namespace remap source %q", from)
		}
		for _, rule := range rules {
			if strings.EqualFold(rule.From, to) && strings.EqualFold(rule.To, from) {
				return nil, fmt.Errorf("namespace remap cycle between %q and %q", from, to)
			}
		}
		seen[fromKey] = true
		rules = append(rules, namespaceremap.Rule{From: from, To: to})
	}
	return rules, nil
}

func parsePackageShims(values []string) ([]PackageShim, error) {
	seen := make(map[string]bool)
	shims := make([]PackageShim, 0, len(values))
	for _, value := range values {
		namespace, sourceRoot, ok := strings.Cut(strings.TrimSpace(value), ":")
		namespace = strings.TrimSpace(namespace)
		sourceRoot = strings.TrimSpace(sourceRoot)
		if !ok || namespace == "" || sourceRoot == "" {
			return nil, fmt.Errorf("invalid package shim %q: expected namespace:path", value)
		}
		key := strings.ToLower(namespace)
		if seen[key] {
			return nil, fmt.Errorf("duplicate package shim namespace %q", namespace)
		}
		seen[key] = true
		shims = append(shims, PackageShim{Namespace: namespace, SourceRoot: sourceRoot})
	}
	return shims, nil
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

func parseManagedPackageDependencies(values []string) ([]ManagedPackageDependency, error) {
	deps := make([]ManagedPackageDependency, 0, len(values))
	for _, value := range values {
		namespace, spec, ok := strings.Cut(strings.TrimSpace(value), ":")
		if !ok {
			return nil, fmt.Errorf("invalid managed package dependency %q: expected namespace:path[:version]", value)
		}
		namespace = strings.TrimSpace(namespace)
		spec = strings.TrimSpace(spec)
		sourceRoot := spec
		artifactPath := ""
		version := ""
		if strings.HasPrefix(spec, "artifact:") && managedPackageArtifactSpecLooksLikePath(strings.TrimSpace(strings.TrimPrefix(spec, "artifact:"))) {
			artifactSpec := strings.TrimSpace(strings.TrimPrefix(spec, "artifact:"))
			if artifactSpec == "" {
				return nil, fmt.Errorf("invalid managed package dependency %q: expected namespace:artifact:path[:version]", value)
			}
			artifactPath = artifactSpec
			if path, requiredVersion, ok := strings.Cut(artifactSpec, ":"); ok {
				artifactPath = strings.TrimSpace(path)
				version = strings.TrimSpace(requiredVersion)
			}
			sourceRoot = ""
		} else if path, requiredVersion, ok := strings.Cut(spec, ":"); ok {
			sourceRoot = strings.TrimSpace(path)
			version = strings.TrimSpace(requiredVersion)
		}
		deps = append(deps, ManagedPackageDependency{
			Namespace:    namespace,
			SourceRoot:   sourceRoot,
			ArtifactPath: artifactPath,
			Version:      version,
		})
	}
	return parseManagedPackageDependencyEntries(deps)
}

func parseManagedPackageDependencyEntries(entries []ManagedPackageDependency) ([]ManagedPackageDependency, error) {
	seen := make(map[string]bool)
	deps := make([]ManagedPackageDependency, 0, len(entries))
	for _, dep := range entries {
		dep.Namespace = strings.TrimSpace(dep.Namespace)
		dep.SourceRoot = strings.TrimSpace(dep.SourceRoot)
		dep.ArtifactPath = strings.TrimSpace(dep.ArtifactPath)
		dep.Version = strings.TrimSpace(dep.Version)
		if dep.Namespace == "" || (dep.SourceRoot == "" && dep.ArtifactPath == "") {
			return nil, fmt.Errorf("invalid managed package dependency %q: namespace and path are required", dep.Namespace)
		}
		if dep.SourceRoot != "" && dep.ArtifactPath != "" {
			return nil, fmt.Errorf("invalid managed package dependency %q: sourceRoot and artifactPath are mutually exclusive", dep.Namespace)
		}
		key := strings.ToLower(dep.Namespace)
		if seen[key] {
			return nil, fmt.Errorf("duplicate managed package dependency namespace %q", dep.Namespace)
		}
		seen[key] = true
		deps = append(deps, dep)
	}
	return deps, nil
}

func managedPackageArtifactSpecLooksLikePath(spec string) bool {
	if spec == "" {
		return true
	}
	return strings.ContainsAny(spec, `/\`) || strings.HasSuffix(strings.ToLower(spec), ".json")
}

func resolveManagedPackageDependencyPaths(cfg *Config, baseDir string) {
	for i := range cfg.Project.ManagedPackageDependencies {
		path := cfg.Project.ManagedPackageDependencies[i].SourceRoot
		if path != "" && !filepath.IsAbs(path) {
			cfg.Project.ManagedPackageDependencies[i].SourceRoot = filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(path)))
		}
		artifactPath := cfg.Project.ManagedPackageDependencies[i].ArtifactPath
		if artifactPath != "" && !filepath.IsAbs(artifactPath) {
			cfg.Project.ManagedPackageDependencies[i].ArtifactPath = filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(artifactPath)))
		}
	}
	for i := range cfg.Project.PackageShims {
		path := cfg.Project.PackageShims[i].SourceRoot
		if path != "" && !filepath.IsAbs(path) {
			cfg.Project.PackageShims[i].SourceRoot = filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(path)))
		}
	}
}
