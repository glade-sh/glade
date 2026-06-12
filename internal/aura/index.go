package aura

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

type Index struct {
	OutApps []OutApp
	byQualified map[string]OutApp
}

func BuildIndex(p project.Project) (Index, error) {
	namespace := strings.TrimSpace(p.Namespace)
	if namespace == "" {
		namespace = "c"
	}
	byQualified := make(map[string]OutApp)
	var apps []OutApp
	for _, group := range groupAuraApps(p.AuraFiles) {
		bundleName := filepath.Base(filepath.Dir(group[0]))
		for _, path := range group {
			if !hasSuffixFold(path, ".app") || hasSuffixFold(path, "-meta.xml") {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return Index{}, err
			}
			app, err := ParseOutApp(bundleName, string(data))
			if err != nil {
				continue
			}
			apps = append(apps, app)
			byQualified[qualifiedKey(namespace, app.Name)] = app
		}
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].Name < apps[j].Name })
	return Index{OutApps: apps, byQualified: byQualified}, nil
}

func (idx Index) OutApp(qualified string) (OutApp, bool) {
	app, ok := idx.byQualified[qualifiedKeyFromQualified(qualified)]
	return app, ok
}

func qualifiedKeyFromQualified(qualified string) string {
	qualified = strings.TrimSpace(qualified)
	if qualified == "" {
		return ""
	}
	parts := strings.SplitN(qualified, ":", 2)
	if len(parts) != 2 {
		return lookupKey(qualified)
	}
	return lookupKey(parts[0] + ":" + parts[1])
}

func qualifiedKey(namespace, name string) string {
	return lookupKey(namespace + ":" + name)
}

func lookupKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func groupAuraApps(paths []string) [][]string {
	byDir := make(map[string][]string)
	for _, path := range paths {
		byDir[filepath.Dir(path)] = append(byDir[filepath.Dir(path)], path)
	}
	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	out := make([][]string, 0, len(dirs))
	for _, dir := range dirs {
		sort.Strings(byDir[dir])
		out = append(out, byDir[dir])
	}
	return out
}

func hasSuffixFold(value, suffix string) bool {
	return len(value) >= len(suffix) && strings.EqualFold(value[len(value)-len(suffix):], suffix)
}
