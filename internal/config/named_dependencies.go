package config

import (
	"errors"
	"fmt"
	"strings"
)

// LocalPackageDependency maps a package name or SFDX package alias to local
// source. Package selects one package directory when Path is an SFDX project.
type LocalPackageDependency struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Package string `json:"package,omitempty"`
	Version string `json:"version,omitempty"`
}

func parseLocalPackageDependencyBlock(lines []string, start, parentIndent int) ([]LocalPackageDependency, int, error) {
	var entries []LocalPackageDependency
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
		if line == "-" || strings.HasPrefix(line, "- ") {
			entries = append(entries, LocalPackageDependency{})
			current = len(entries) - 1
			line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
			if line == "" {
				continue
			}
		} else if current < 0 {
			return nil, lineNo, fmt.Errorf("glade.yml:%d: expected project dependency list item", lineNo+1)
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, lineNo, fmt.Errorf("glade.yml:%d: expected key: value", lineNo+1)
		}
		key = strings.TrimSpace(key)
		value = trimScalar(value)
		switch key {
		case "name":
			entries[current].Name = value
		case "path":
			entries[current].Path = value
		case "package":
			entries[current].Package = value
		case "version":
			entries[current].Version = value
		default:
			return nil, lineNo, fmt.Errorf("glade.yml:%d: unsupported project dependency key %q", lineNo+1, key)
		}
	}
	return entries, len(lines), nil
}

func validateLocalPackageDependencies(entries []LocalPackageDependency) ([]LocalPackageDependency, error) {
	seen := make(map[string]string, len(entries))
	for _, dep := range entries {
		name := strings.TrimSpace(dep.Name)
		if name == "" || strings.TrimSpace(dep.Path) == "" {
			return nil, errors.New("project dependency requires a nonempty name and path")
		}
		key := normalizeLocalPackageDependencyName(name)
		if key == "" {
			return nil, errors.New("project dependency requires a nonempty name")
		}
		if previousPath, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate project dependency name %q maps to both %q and %q", name, previousPath, dep.Path)
		}
		seen[key] = dep.Path
	}
	return entries, nil
}

func normalizeLocalPackageDependencyName(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.Index(name, "@"); i >= 0 {
		name = name[:i]
	}
	return strings.ToLower(strings.TrimSpace(name))
}
