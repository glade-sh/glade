package apextest

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/typesys"
)

func (cache *sourceCache) seedProjectSnapshot(snapshot *project.Project) {
	if cache == nil || snapshot == nil {
		return
	}
	captured := *snapshot
	cache.mu.Lock()
	cache.capturedProject = &captured
	cache.mu.Unlock()
}

func (cache *sourceCache) captureProjectRuntimeMetadata(index typesys.Index, generation *runtimeMetadataGeneration) error {
	bodies := make(map[string]string)
	var capturedProject *project.Project
	if index.Project.Root != "" {
		if loaded, err := cache.capturedProjectSnapshot(index.Project.Root); err == nil {
			capturedProject = &loaded
			paths := append([]string(nil), loaded.ListViewFiles...)
			paths = append(paths, resource.LayoutMetadataFiles(loaded)...)
			paths = append(paths, loaded.FlowFiles...)
			paths = append(paths, loaded.NetworkFiles...)
			paths = append(paths, loaded.PermissionSetGroupFiles...)
			paths = append(paths, resource.FeatureParameterMetadataFiles(loaded)...)
			paths = append(paths, resource.SetupRecordMetadataFiles(loaded)...)
			paths = append(paths, project.EmailServiceMetadataFiles(loaded)...)
			for _, path := range paths {
				data, err := readProjectMetadataFile(loaded, path)
				if err != nil {
					return err
				}
				bodies[path] = string(data)
				generation.inputs[path] = typesys.ApexMetadataInput{Present: true, Digest: sha256.Sum256(data)}
			}
		}
	}
	var loadedRoots []string
	if capturedProject != nil {
		loadedRoots = loadedProjectRoots(*capturedProject)
	}
	units, err := captureRuntimeSourceUnitsInRoots(index, generation, bodies, loadedRoots)
	if err != nil {
		return err
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.projectMetadataBodies = bodies
	cache.projectMetadataPrepared = true
	cache.capturedProject = capturedProject
	cache.runtimeSourceUnits = units
	if cache.capturedMetadata == nil {
		cache.capturedMetadata = make(map[string]typesys.ApexMetadataInput)
	}
	for path := range generation.inputs {
		cache.capturedMetadata[path] = generation.inputs[path]
	}
	return nil
}

func (cache *sourceCache) readCapturedProjectMetadata(path string) ([]byte, error) {
	cache.mu.RLock()
	body, exists := cache.projectMetadataBodies[path]
	prepared := cache.projectMetadataPrepared
	capturedProject := cache.capturedProject
	cache.mu.RUnlock()
	if exists {
		return []byte(body), nil
	}
	if prepared {
		return nil, incompleteSourceSnapshotError("missing captured project metadata " + path)
	}
	if capturedProject == nil {
		return nil, fmt.Errorf("metadata path %q has no indexed source root", path)
	}
	return readProjectMetadataFile(*capturedProject, path)
}

func (cache *sourceCache) capturedProjectSnapshot(root string) (project.Project, error) {
	cache.mu.RLock()
	captured := cache.capturedProject
	cache.mu.RUnlock()
	if captured != nil && captured.Root == root {
		return *captured, nil
	}
	loaded, err := project.Load(root)
	if err == nil {
		cache.seedProjectSnapshot(&loaded)
	}
	return loaded, err
}

// sourceRootPath preserves relative path semantics while checking containment
// using absolute, cleaned paths. A sibling with the same prefix is not a child.
func sourceRootPath(root, path string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("path %q has no indexed source root", path)
	}
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside source root %q", path, root)
	}
	return filepath.Clean(path), nil
}

// projectSourceRoots lists the project root followed by every resolved package
// directory that sits outside it. The loader resolves packageDirectories and
// project.packageDirs entries such as "../shared" relative to the root, so
// their files are indexed and must pass the same containment check. Package
// directories inside the root are already covered by the root itself.
func projectSourceRoots(p *project.Project) []string {
	if p == nil || strings.TrimSpace(p.Root) == "" {
		return nil
	}
	roots := []string{p.Root}
	for _, pkgRoot := range append(p.PackageRoots(), p.ExtraMetadataDirs...) {
		if _, err := sourceRootPath(p.Root, pkgRoot); err != nil {
			roots = append(roots, pkgRoot)
		}
	}
	return roots
}

// loadedProjectRoots lists the root of the project and of every loaded
// dependency or shim project beneath it, the same set the index reads from.
func loadedProjectRoots(snapshot project.Project) []string {
	seen := make(map[*project.Project]bool)
	var roots []string
	var visit func(*project.Project)
	visit = func(p *project.Project) {
		if p == nil || seen[p] {
			return
		}
		seen[p] = true
		roots = append(roots, projectSourceRoots(p)...)
		for _, dep := range p.ManagedPackageDependencies {
			if dep.Status == "loaded" {
				visit(dep.Project)
			}
		}
		for _, shim := range p.PackageShims {
			if shim.Status == "loaded" {
				visit(shim.Project)
			}
		}
	}
	visit(&snapshot)
	return roots
}

// containingSourceRoot returns the deepest loaded root that contains path. It
// falls back to primary so a path outside every root is rejected by the
// containment check against the primary root.
func containingSourceRoot(path, primary string, loaded []string) string {
	best := primary
	bestDepth := -1
	for _, candidate := range append([]string{primary}, loaded...) {
		if _, err := sourceRootPath(candidate, path); err != nil {
			continue
		}
		if depth := len(filepath.Clean(candidate)); depth > bestDepth {
			best, bestDepth = candidate, depth
		}
	}
	return best
}

func readSourceRootFile(root, path string) ([]byte, error) {
	path, err := sourceRootPath(root, path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Clean(path))
}

func readProjectMetadataFile(snapshot project.Project, path string) ([]byte, error) {
	seen := make(map[*project.Project]bool)
	var sourceRoot func(*project.Project) string
	sourceRoot = func(p *project.Project) string {
		if p == nil || seen[p] {
			return ""
		}
		seen[p] = true
		for _, root := range projectSourceRoots(p) {
			if _, err := sourceRootPath(root, path); err == nil {
				return root
			}
		}
		for _, dep := range p.ManagedPackageDependencies {
			if dep.Status == "loaded" {
				if root := sourceRoot(dep.Project); root != "" {
					return root
				}
			}
		}
		for _, shim := range p.PackageShims {
			if shim.Status == "loaded" {
				if root := sourceRoot(shim.Project); root != "" {
					return root
				}
			}
		}
		return ""
	}
	root := sourceRoot(&snapshot)
	if root == "" {
		return nil, fmt.Errorf("metadata path %q is outside source root %q", path, snapshot.Root)
	}
	return readSourceRootFile(root, path)
}
