package project

import "path/filepath"

// EffectiveOrgShapeFeatures reports the org shape of the effective project:
// the project root plus every loaded dependency view (dependencyPaths and SFDX
// package dependencies resolved to source, managed-package sources and package
// shims, at any depth). Those views deploy into the same org, so a feature that
// any of them declares or evidences belongs to the org. The root's features
// come first, so a project without contributing dependencies keeps exactly
// OrgShapeFeatures(p.Root).
func EffectiveOrgShapeFeatures(p Project) []string {
	var features []string
	for _, root := range effectiveOrgShapeRoots(p) {
		features = append(features, OrgShapeFeatures(root)...)
	}
	return dedupeStrings(features)
}

// EffectiveOrgShapeInputFiles lists OrgShapeInputFiles for every root that
// EffectiveOrgShapeFeatures reads, so dependency evidence edits invalidate
// caches keyed on the org shape.
func EffectiveOrgShapeInputFiles(p Project) []string {
	var paths []string
	for _, root := range effectiveOrgShapeRoots(p) {
		paths = append(paths, OrgShapeInputFiles(root)...)
	}
	return dedupeStrings(paths)
}

func effectiveOrgShapeRoots(p Project) []string {
	var roots []string
	seenRoots := map[string]bool{}
	seenProjects := map[*Project]bool{}
	var visit func(*Project)
	visit = func(current *Project) {
		if current == nil || seenProjects[current] {
			return
		}
		seenProjects[current] = true
		if current.Root != "" {
			root := filepath.Clean(current.Root)
			if !seenRoots[root] {
				seenRoots[root] = true
				roots = append(roots, current.Root)
			}
		}
		for _, dep := range current.ManagedPackageDependencies {
			if dep.Status == "loaded" {
				visit(dep.Project)
			}
		}
		for _, shim := range current.PackageShims {
			if shim.Status == "loaded" {
				visit(shim.Project)
			}
		}
	}
	visit(&p)
	return roots
}
