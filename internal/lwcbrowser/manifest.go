package lwcbrowser

import (
	"path"
	"strings"

	"github.com/glade-sh/glade/internal/aura"
	"github.com/glade-sh/glade/internal/lwc/compile"
)

type ModuleEntry struct {
	URL string
	Tag string
}

type Manifest struct {
	Modules map[string]ModuleEntry
}

func ManifestFromCompile(m compile.Manifest, urlPrefix string) Manifest {
	if urlPrefix == "" {
		urlPrefix = "/lightning/modules"
	}
	out := Manifest{Modules: make(map[string]ModuleEntry, len(m.Modules))}
	for qualified, entry := range m.Modules {
		url := urlPrefix + "/" + entry.ModuleKey + "/" + path.Base(entry.ModuleKey) + ".js"
		out.Modules[normalizeQualified(qualified)] = ModuleEntry{
			URL: url,
			Tag: entry.Tag,
		}
	}
	return out
}

func (m Manifest) Resolve(qualified string) (ModuleEntry, bool) {
	entry, ok := m.Modules[normalizeQualified(qualified)]
	return entry, ok
}

func ApplyAuraLWCPassthroughAliases(m Manifest, passthroughs []aura.LWCPassthrough, namespace string) {
	if namespace == "" {
		namespace = "c"
	}
	for _, pt := range passthroughs {
		target, ok := m.Modules[normalizeQualified(pt.Target)]
		if !ok {
			continue
		}
		alias := normalizeQualified(namespace + ":" + pt.AuraName)
		if _, exists := m.Modules[alias]; exists {
			continue
		}
		m.Modules[alias] = target
	}
}

func normalizeQualified(qualified string) string {
	qualified = strings.TrimSpace(qualified)
	qualified = strings.ReplaceAll(qualified, "/", ":")
	return strings.ToLower(qualified)
}

func ModuleFilePath(outDir, moduleKey string) string {
	base := path.Base(moduleKey)
	return path.Join(outDir, moduleKey, base+".js")
}

func ModuleSiblingPath(outDir, moduleKey, fileName string) string {
	return path.Join(outDir, moduleKey, fileName)
}

const lightningModulesPrefix = "/lightning/modules/"

// LocalLWCImportMap returns import-map entries for compiled LWC bundles.
// The LWC compiler emits package-local imports as c/componentName even in
// namespaced packages; map those aliases to the compiled module URLs.
func LocalLWCImportMap(namespace string, m Manifest) map[string]string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = "c"
	}
	imports := make(map[string]string)
	for _, entry := range m.Modules {
		moduleNS, component, ok := moduleSpecFromURL(entry.URL)
		if !ok {
			continue
		}
		spec := moduleNS + "/" + component
		imports[spec] = entry.URL
		if !strings.EqualFold(moduleNS, "c") {
			imports["c/"+component] = entry.URL
		}
	}
	return imports
}

func moduleSpecFromURL(url string) (namespace, component string, ok bool) {
	url = strings.TrimSpace(url)
	if !strings.HasPrefix(url, lightningModulesPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(url, lightningModulesPrefix)
	parts := strings.Split(rest, "/")
	if len(parts) < 3 {
		return "", "", false
	}
	component = parts[len(parts)-2]
	if component == "" {
		return "", "", false
	}
	return parts[0], component, true
}
