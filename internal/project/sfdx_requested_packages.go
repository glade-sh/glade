package project

import (
	"sort"
	"strings"
)

// A sibling manifest may supply several packages while its default directory
// holds development-only metadata. Load the packages requested by the consumer
// and their dependencies within this manifest, in manifest order.
func requestedDependencyPackageDirectories(dirs []PackageDirectory, requested []string) []PackageDirectory {
	wanted := make(map[string]bool, len(requested))
	for _, name := range requested {
		if name = normalizeSFDXPackageName(name); name != "" {
			wanted[name] = true
		}
	}
	selected := make([]bool, len(dirs))
	for changed := true; changed; {
		changed = false
		for i, dir := range dirs {
			if selected[i] || !wanted[normalizeSFDXPackageName(dir.Package)] {
				continue
			}
			selected[i] = true
			changed = true
			for _, dependency := range dir.Dependencies {
				if name := normalizeSFDXPackageName(dependency.Package); name != "" {
					wanted[name] = true
				}
			}
		}
	}
	var result []PackageDirectory
	for i, dir := range dirs {
		if selected[i] {
			result = append(result, dir)
		}
	}
	return result
}

// Revisiting another package in a supplier is not a dependency cycle. Compare
// the selected source roots so request ordering does not change cycle identity.
func dependencyPackageViewKey(root string, dirs []PackageDirectory) string {
	paths := packageRoots(root, dirs, false)
	sort.Strings(paths)
	return "view\x00" + root + "\x00" + strings.Join(paths, "\x00")
}
