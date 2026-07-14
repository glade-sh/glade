package watch

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

// Scope is the exact set of directory trees and individual files observed by
// a watcher.
type Scope struct {
	Roots                 []string
	Files                 []string
	Topology              []string
	TopologyOwners        map[string][]string
	ExcludedRoots         []string
	ExclusionExceptions   []string
	TopologyBase          string
	ScanTopologyAncestors bool
}

func NormalizeScope(scope Scope) Scope {
	topologyBase := cleanOptionalAbsPath(scope.TopologyBase)
	scanTopologyAncestors := scope.ScanTopologyAncestors
	topology := append([]string(nil), scope.Topology...)
	for _, root := range normalizeScopePaths(scope.Roots) {
		_, components, _ := resolveScopeRoot(root, topologyBase, scanTopologyAncestors)
		topology = append(topology, components...)
	}
	roots := normalizeScopeRoots(scope.Roots, topologyBase, scanTopologyAncestors)
	files := normalizeScopePaths(scope.Files)
	topology = normalizeScopePaths(topology)
	excludedRoots := normalizeScopeRoots(scope.ExcludedRoots, topologyBase, scanTopologyAncestors)
	exclusionExceptions := normalizeScopeRoots(scope.ExclusionExceptions, topologyBase, scanTopologyAncestors)

	keptRoots := roots[:0]
	for _, root := range roots {
		redundant := false
		for _, parent := range keptRoots {
			if pathWithin(root, parent) || sameExistingFile(root, parent) {
				redundant = true
				break
			}
		}
		if !redundant {
			keptRoots = append(keptRoots, root)
		}
	}
	keptFiles := files[:0]
	for _, file := range files {
		covered := false
		for _, root := range keptRoots {
			if isRootConfigPath(file, root) {
				covered = true
				break
			}
		}
		if !covered {
			keptFiles = append(keptFiles, file)
		}
	}
	if len(keptFiles) == 0 {
		keptFiles = nil
	}
	if len(topology) == 0 {
		topology = nil
	}
	topologyOwners := normalizeTopologyOwners(scope.TopologyOwners, topology)
	keptExcluded := excludedRoots[:0]
	for _, excluded := range excludedRoots {
		allowed := false
		// An explicitly loaded root remains authoritative even when a broader
		// root collapses it out of keptRoots. This can happen when the same
		// package is both a direct dependency and nested below another direct
		// dependency.
		for _, root := range roots {
			if excluded == root || sameExistingFile(excluded, root) {
				allowed = true
				break
			}
		}
		if !allowed {
			keptExcluded = append(keptExcluded, excluded)
		}
	}
	if len(keptExcluded) == 0 {
		keptExcluded = nil
	}
	keptExceptions := exclusionExceptions[:0]
	for _, exception := range exclusionExceptions {
		for _, excluded := range keptExcluded {
			if pathWithin(exception, excluded) && exception != excluded {
				keptExceptions = append(keptExceptions, exception)
				break
			}
		}
	}
	if len(keptExceptions) == 0 {
		keptExceptions = nil
	}
	// Ancestor scanning is a one-shot normalization input. Roots are physical
	// and topology endpoints explicit after this pass; carrying the flag into
	// later Config normalization would climb into ambient volume aliases.
	return Scope{Roots: keptRoots, Files: keptFiles, Topology: topology, TopologyOwners: topologyOwners, ExcludedRoots: keptExcluded, ExclusionExceptions: keptExceptions, TopologyBase: topologyBase}
}

func normalizeTopologyOwners(owners map[string][]string, topology []string) map[string][]string {
	if len(owners) == 0 || len(topology) == 0 {
		return nil
	}
	allowed := make(map[string]bool, len(topology))
	for _, endpoint := range topology {
		allowed[endpoint] = true
	}
	normalized := make(map[string][]string)
	for endpoint, endpointOwners := range owners {
		endpoint = cleanAbsPath(endpoint)
		if !allowed[endpoint] {
			continue
		}
		for _, owner := range endpointOwners {
			if strings.TrimSpace(owner) == "" {
				continue
			}
			normalized[endpoint] = append(normalized[endpoint], cleanAbsPath(owner))
		}
		sort.Strings(normalized[endpoint])
		normalized[endpoint] = slices.Compact(normalized[endpoint])
		if len(normalized[endpoint]) == 0 {
			delete(normalized, endpoint)
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeScopeRoots(paths []string, topologyBase string, scanTopologyAncestors bool) []string {
	roots := normalizeScopePaths(paths)
	for i, root := range roots {
		physical, _, err := resolveScopeRoot(root, topologyBase, scanTopologyAncestors)
		if err == nil {
			roots[i] = physical
		}
	}
	sort.Strings(roots)
	canonical := make([]string, 0, len(roots))
	for _, root := range roots {
		aliasIndex := -1
		for i, existing := range canonical {
			if sameExistingFile(root, existing) {
				aliasIndex = i
				break
			}
		}
		if aliasIndex < 0 {
			canonical = append(canonical, root)
			continue
		}
		if isSymlink(canonical[aliasIndex]) && !isSymlink(root) {
			canonical[aliasIndex] = root
		}
	}
	sort.Strings(canonical)
	return canonical
}

func resolveScopeRoot(path, topologyBase string, scanTopologyAncestors bool) (string, []string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	abs = filepath.Clean(abs)
	base := cleanOptionalAbsPath(topologyBase)
	if base == "" || !pathWithin(abs, base) {
		base = fallbackTopologyBoundary(abs, filepath.Dir, isSymlink, scanTopologyAncestors)
	}
	return resolveScopePath(abs, base, topologyBase, make(map[string]bool))
}

func fallbackTopologyBoundary(path string, dir func(string) string, symlink func(string) bool, scanAncestors bool) string {
	if scanAncestors {
		outermost := ""
		for candidate := path; ; candidate = dir(candidate) {
			if symlink(candidate) {
				outermost = dir(candidate)
			}
			if parent := dir(candidate); parent == candidate {
				break
			}
		}
		if outermost != "" {
			return outermost
		}
	}
	parent := dir(path)
	boundary := dir(parent)
	if boundary == parent {
		return parent
	}
	return boundary
}

func resolveScopePath(path, base, topologyBase string, seen map[string]bool) (string, []string, error) {
	remainder, err := filepath.Rel(base, path)
	if err != nil {
		return "", nil, err
	}
	if remainder == "." {
		if !isSymlink(path) {
			return filepath.Clean(path), nil, nil
		}
		return resolveScopeSymlink(path, filepath.Dir(path), topologyBase, seen)
	}
	lexical := base
	resolved := lexical
	var topology []string
	for _, component := range strings.Split(remainder, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		lexical = filepath.Join(lexical, component)
		if isSymlink(lexical) {
			targetResolved, targetTopology, err := resolveScopeSymlink(lexical, resolved, topologyBase, seen)
			if err != nil {
				return "", nil, err
			}
			topology = append(topology, targetTopology...)
			resolved = targetResolved
			continue
		}
		resolved = filepath.Join(resolved, component)
	}
	return filepath.Clean(resolved), topology, nil
}

func resolveScopeSymlink(path, physicalParent, topologyBase string, seen map[string]bool) (string, []string, error) {
	path = filepath.Clean(path)
	if seen[path] {
		return "", nil, errors.New("symlink cycle")
	}
	seen[path] = true
	target, err := os.Readlink(path)
	if err != nil {
		return "", nil, err
	}
	if filepath.IsAbs(target) {
		target = filepath.Clean(target)
	} else {
		target = filepath.Clean(filepath.Join(physicalParent, target))
	}
	targetBase := scopeTargetBoundary(target, physicalParent, topologyBase)
	resolved, topology, err := resolveScopePath(target, targetBase, topologyBase, seen)
	if err != nil {
		// Keep the outer endpoint observable even when a nested target cannot
		// be traversed (for example, an absolute target on another volume).
		return target, []string{path}, nil
	}
	return resolved, append([]string{path}, topology...), nil
}

func scopeTargetBoundary(target, physicalParent, topologyBase string) string {
	base := cleanOptionalAbsPath(topologyBase)
	if base != "" {
		if pathWithin(target, base) {
			return base
		}
		// A lexical workspace prefix can itself be a symlink. Find its
		// physical counterpart without recording ambient platform aliases.
		for ancestor := filepath.Clean(target); ; ancestor = filepath.Dir(ancestor) {
			if sameExistingFile(ancestor, base) {
				return ancestor
			}
			if parent := filepath.Dir(ancestor); parent == ancestor {
				break
			}
		}
	}
	// The nearest common project/source container is the generic fallback for
	// an explicit absolute target outside TopologyBase. Scanning from it sees
	// intermediate target symlinks without special-casing OS path prefixes.
	if common := commonPathAncestor([]string{physicalParent, target}); common != "" {
		return common
	}
	for ancestor := filepath.Clean(target); ; ancestor = filepath.Dir(ancestor) {
		if isSymlink(ancestor) {
			return filepath.Dir(ancestor)
		}
		if parent := filepath.Dir(ancestor); parent == ancestor {
			break
		}
	}
	return filepath.Dir(target)
}

func cleanOptionalAbsPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return cleanAbsPath(path)
}

func normalizeScopePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		abs = filepath.Clean(abs)
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		out = append(out, abs)
	}
	sort.Strings(out)
	return out
}

func sameExistingFile(a, b string) bool {
	aInfo, aErr := os.Stat(a)
	bInfo, bErr := os.Stat(b)
	return aErr == nil && bErr == nil && os.SameFile(aInfo, bInfo)
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ProjectScope returns the project roots and authoritative configuration files
// that contributed directly to p. Artifact and transitive dependencies are not
// source inputs and are intentionally excluded.
func ProjectScope(requestedRoot string, p project.Project) Scope {
	return projectScope(requestedRoot, p, Scope{})
}

// ProjectScopeWithPrevious retains an explicit symlink topology endpoint only
// while the candidate project still configures that source root. This keeps a
// deleted alias observable without retaining roots removed from configuration.
func ProjectScopeWithPrevious(requestedRoot string, p project.Project, previous Scope) Scope {
	return projectScope(requestedRoot, p, previous)
}

func projectScope(requestedRoot string, p project.Project, previous Scope) Scope {
	roots := []string{requestedRoot, p.Root}
	var excludedRoots []string
	exclusionExceptions := []string{requestedRoot, p.Root}
	visitedProjects := make(map[*project.Project]bool)
	configuredSourceRoots := make(map[string]struct{})
	incompleteSourceRoots := make(map[string]bool)
	for _, dependency := range p.ManagedPackageDependencies {
		if strings.TrimSpace(dependency.SourceRoot) != "" && dependency.ArtifactPath == "" {
			owner := cleanAbsPath(dependency.SourceRoot)
			configuredSourceRoots[owner] = struct{}{}
			if dependency.Status != "loaded" || dependency.Project == nil {
				incompleteSourceRoots[owner] = true
			}
		}
		if dependency.Status != "loaded" || dependency.Project == nil || strings.TrimSpace(dependency.SourceRoot) == "" || dependency.ArtifactPath != "" {
			excludedRoots = appendProjectSourceRoots(excludedRoots, dependency.SourceRoot, dependency.Project, visitedProjects)
			continue
		}
		roots = append(roots, dependency.SourceRoot)
		roots = append(roots, dependency.Project.Root)
		exclusionExceptions = append(exclusionExceptions, dependency.SourceRoot, dependency.Project.Root)
		excludedRoots = appendNestedProjectSources(excludedRoots, *dependency.Project, visitedProjects)
	}
	for _, shim := range p.PackageShims {
		if strings.TrimSpace(shim.SourceRoot) != "" {
			owner := cleanAbsPath(shim.SourceRoot)
			configuredSourceRoots[owner] = struct{}{}
			if shim.Status != "loaded" || shim.Project == nil {
				incompleteSourceRoots[owner] = true
			}
		}
		if shim.Status != "loaded" || shim.Project == nil {
			excludedRoots = appendProjectSourceRoots(excludedRoots, shim.SourceRoot, shim.Project, visitedProjects)
			continue
		}
		roots = append(roots, shim.SourceRoot)
		roots = append(roots, shim.Project.Root)
		exclusionExceptions = append(exclusionExceptions, shim.SourceRoot, shim.Project.Root)
		excludedRoots = appendNestedProjectSources(excludedRoots, *shim.Project, visitedProjects)
	}

	files := make([]string, 0, len(roots)*2+1)
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		files = append(files, filepath.Join(root, "sfdx-project.json"), filepath.Join(root, "glade.yml"))
	}
	files = append(files, ancestorGladeConfigCandidates(requestedRoot)...)
	scanTopologyAncestors := !pathVolumesMatch(normalizeScopePaths(roots), filepath.VolumeName)
	topologyBase := commonPathAncestor(roots)
	var topology []string
	topologyOwners := make(map[string][]string)
	discovered := make(map[string]map[string]bool)
	for owner := range configuredSourceRoots {
		_, endpoints, err := resolveScopeRoot(owner, topologyBase, scanTopologyAncestors)
		if err != nil {
			continue
		}
		for _, endpoint := range endpoints {
			endpoint = cleanAbsPath(endpoint)
			topology = append(topology, endpoint)
			topologyOwners[endpoint] = append(topologyOwners[endpoint], owner)
			if discovered[owner] == nil {
				discovered[owner] = make(map[string]bool)
			}
			discovered[owner][endpoint] = true
		}
	}
	previousOwners := make(map[string][]string, len(previous.Topology))
	for _, endpoint := range previous.Topology {
		endpoint = cleanAbsPath(endpoint)
		for _, rawOwner := range previous.TopologyOwners[endpoint] {
			if strings.TrimSpace(rawOwner) == "" {
				continue
			}
			owner := cleanAbsPath(rawOwner)
			if _, configured := configuredSourceRoots[owner]; configured {
				previousOwners[endpoint] = append(previousOwners[endpoint], owner)
			}
		}
		if len(previousOwners[endpoint]) > 0 {
			continue
		}
		// Backward-compatible ownership for a lexical endpoint that is itself
		// an ancestor of a still-configured source root.
		for owner := range configuredSourceRoots {
			if pathWithin(owner, endpoint) {
				previousOwners[endpoint] = append(previousOwners[endpoint], owner)
			}
		}
	}
	for endpoint, owners := range previousOwners {
		for _, owner := range owners {
			if discovered[owner][endpoint] {
				continue
			}
			if incompleteSourceRoots[owner] {
				topology = append(topology, endpoint)
				topologyOwners[endpoint] = append(topologyOwners[endpoint], owner)
			}
		}
	}
	return NormalizeScope(Scope{Roots: roots, Files: files, Topology: topology, TopologyOwners: topologyOwners, ExcludedRoots: excludedRoots, ExclusionExceptions: exclusionExceptions, TopologyBase: topologyBase, ScanTopologyAncestors: scanTopologyAncestors})
}

func commonPathAncestor(paths []string) string {
	normalized := normalizeScopePaths(paths)
	if len(normalized) == 0 {
		return ""
	}
	if !pathVolumesMatch(normalized, filepath.VolumeName) {
		return ""
	}
	common := normalized[0]
	for _, path := range normalized[1:] {
		for !pathWithin(path, common) {
			parent := filepath.Dir(common)
			if parent == common {
				return common
			}
			common = parent
		}
	}
	return common
}

func pathVolumesMatch(paths []string, volumeName func(string) string) bool {
	if len(paths) < 2 {
		return true
	}
	volume := volumeName(paths[0])
	for _, path := range paths[1:] {
		if !strings.EqualFold(volume, volumeName(path)) {
			return false
		}
	}
	return true
}

func appendNestedProjectSources(out []string, p project.Project, visited map[*project.Project]bool) []string {
	for _, dependency := range p.ManagedPackageDependencies {
		out = appendProjectSourceRoots(out, dependency.SourceRoot, dependency.Project, visited)
	}
	for _, shim := range p.PackageShims {
		out = appendProjectSourceRoots(out, shim.SourceRoot, shim.Project, visited)
	}
	return out
}

func appendProjectSourceRoots(out []string, sourceRoot string, nested *project.Project, visited map[*project.Project]bool) []string {
	if strings.TrimSpace(sourceRoot) != "" {
		out = append(out, sourceRoot)
	}
	if nested == nil {
		return out
	}
	if visited[nested] {
		return out
	}
	visited[nested] = true
	if strings.TrimSpace(nested.Root) != "" {
		out = append(out, nested.Root)
	}
	return appendNestedProjectSources(out, *nested, visited)
}

func cleanAbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func ancestorGladeConfigCandidates(start string) []string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil
	}
	var candidates []string
	for {
		candidates = append(candidates, filepath.Join(dir, "glade.yml"))
		parent := filepath.Dir(dir)
		if parent == dir {
			return candidates
		}
		dir = parent
	}
}
