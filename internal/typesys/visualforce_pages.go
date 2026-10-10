package typesys

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

// Hand-built snippet projects can omit metadata discovery. An empty inventory
// is authoritative only when the caller supplied a package or page inventory.
// Artifact-only dependencies do not currently carry Visualforce page names.
func projectVisualforcePagesKnown(p project.Project) bool {
	if p.PackageDirectories == nil && p.VisualforcePageFiles == nil {
		return false
	}
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status == "loaded" && (dep.Project == nil || !projectVisualforcePagesKnown(*dep.Project)) {
			return false
		}
	}
	return true
}

// Page token identity comes from the discovered metadata filenames. Retain it
// in the index generation so semantic analysis never reloads a live project.
// Markup and controller validation remain the Visualforce loader's concern.
func projectVisualforcePageNames(p project.Project) []string {
	names := make(map[string]bool)
	var visit func(project.Project, string)
	visit = func(p project.Project, namespace string) {
		for _, dep := range p.ManagedPackageDependencies {
			if dep.Status == "loaded" && dep.Project != nil {
				visit(*dep.Project, dep.Namespace)
			}
		}
		for _, path := range p.VisualforcePageFiles {
			name := strings.ToLower(filepath.Base(path))
			switch {
			case strings.HasSuffix(name, ".page-meta.xml"):
				name = strings.TrimSuffix(name, ".page-meta.xml")
			case strings.HasSuffix(name, ".page"):
				name = strings.TrimSuffix(name, ".page")
			default:
				continue
			}
			if name == "" {
				continue
			}
			names[name] = true
			if namespace != "" && !strings.Contains(name, "__") {
				names[strings.ToLower(namespace)+"__"+name] = true
			}
		}
	}
	visit(p, p.Namespace)
	var result []string
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
