package project

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// maxExtraScratchDepth bounds the walk below <root>/config.
const maxExtraScratchDepth = 3

// StaticOnlyShapeFeaturePrefix marks a feature that only unreferenced config
// definitions enable. It travels in the captured feature list so the static
// object-availability gate can see it. It is never a runtime org feature:
// storage drops it before applying a shape.
const StaticOnlyShapeFeaturePrefix = "__glade_static_only_shape_"

// StaticOnlyShapeFeatures lists the features that definitions kept under
// <root>/config enable but no named definition does, each wrapped in
// StaticOnlyShapeFeaturePrefix. A project that keeps several definitions may
// deploy sources into any of them, so the static gate cannot establish that an
// object those features provide is absent (an unspecified shape is
// ungated). The runtime org follows the named definitions only.
func StaticOnlyShapeFeatures(p Project) []string {
	var out []string
	for _, root := range effectiveOrgShapeRoots(p) {
		named := make(map[string]bool)
		for _, feature := range OrgShapeFeatures(root) {
			named[strings.ToLower(feature)] = true
		}
		for _, path := range extraScratchDefinitionFiles(root) {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var def scratchOrgDefinition
			if json.Unmarshal(data, &def) != nil {
				continue
			}
			for _, feature := range scratchDefinitionFeatures(def) {
				if !named[strings.ToLower(feature)] {
					out = append(out, StaticOnlyShapeFeaturePrefix+feature)
				}
			}
		}
	}
	return dedupeStrings(out)
}

// extraScratchDefinitionFiles lists scratch definitions kept under
// <root>/config (for example config/scratch-orgs/advanced-scratch-def.json)
// that no sfdx-project.json definitionFile or CumulusCI task names. See
// StaticOnlyShapeFeatures for how they are used.
func extraScratchDefinitionFiles(root string) []string {
	if root == "" {
		return nil
	}
	base := filepath.Join(root, "config")
	var paths []string
	_ = filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil {
			return nil
		}
		if entry.IsDir() {
			if rel != "." && (strings.Count(rel, string(filepath.Separator)) >= maxExtraScratchDepth || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() && strings.HasSuffix(strings.ToLower(entry.Name()), "scratch-def.json") {
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	return paths
}
