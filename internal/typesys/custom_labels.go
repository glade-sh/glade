package typesys

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

// Retain label names in the immutable index generation. Snippet projects may
// omit metadata discovery, and namespaced dependency visibility is not known
// from a root project's label files alone.
func projectCustomLabels(p project.Project) ([]string, bool) {
	if strings.TrimSpace(p.Namespace) != "" || len(p.NamespaceRemaps) != 0 || len(p.ManagedPackageDependencies) != 0 || len(p.PackageShims) != 0 || len(p.DependencyDiagnostics) != 0 || (p.PackageDirectories == nil && p.LabelFiles == nil) {
		return nil, false
	}
	names := make(map[string]bool)
	for _, path := range p.LabelFiles {
		if !filepath.IsAbs(path) {
			path = filepath.Join(p.Root, path)
		}
		data, err := os.ReadFile(path) // #nosec G304 -- discovered project metadata path.
		if err != nil {
			return nil, false
		}
		var labels struct {
			XMLName xml.Name `xml:"CustomLabels"`
			Labels  []struct {
				FullName string `xml:"fullName"`
			} `xml:"labels"`
		}
		if err := xml.Unmarshal(data, &labels); err != nil {
			return nil, false
		}
		for _, label := range labels.Labels {
			name := strings.TrimSpace(label.FullName)
			if name == "" {
				return nil, false
			}
			names[strings.ToLower(name)] = true
		}
	}
	var result []string
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, true
}
