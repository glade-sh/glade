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
			generation.standardDuplicateRules = loaded.StandardDuplicateRules
			paths := append([]string(nil), loaded.ListViewFiles...)
			paths = append(paths, resource.LayoutMetadataFiles(loaded)...)
			paths = append(paths, resource.FolderItemMetadataFiles(loaded)...)
			paths = append(paths, loaded.FlowFiles...)
			paths = append(paths, loaded.NetworkFiles...)
			paths = append(paths, loaded.PaymentGatewayProviderFiles...)
			paths = append(paths, resource.NamedCredentialMetadataFiles(loaded)...)
			paths = append(paths, loaded.PermissionSetGroupFiles...)
			paths = append(paths, loaded.ProfileFiles...)
			paths = append(paths, dependencyAdminProfileFiles(loaded)...)
			paths = append(paths, resource.FeatureParameterMetadataFiles(loaded)...)
			paths = append(paths, resource.StandardValueSetTranslationMetadataFiles(loaded)...)
			paths = append(paths, resource.SetupRecordMetadataFiles(loaded)...)
			paths = append(paths, project.EmailServiceMetadataFiles(loaded)...)
			if err := captureProviderAdapterGeneration(loaded, paths, bodies, generation, func(path string) ([]byte, error) { return readProjectMetadataFile(loaded, path) }); err != nil {
				return err
			}
			if err := captureVisualforceRuntimeInputs(loaded, generation); err != nil {
				return err
			}
		}
	}
	var loadedRoots []string
	readMarker := os.ReadFile
	if capturedProject != nil {
		loadedRoots = loadedProjectRoots(*capturedProject)
		readMarker = capturedProject.ReadFile
	}
	units, err := captureRuntimeSourceUnitsInRoots(index, generation, bodies, loadedRoots, readMarker)
	if err != nil {
		return err
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.projectMetadataBodies = bodies
	cache.projectMetadataPrepared = true
	cache.capturedProject = capturedProject
	cache.runtimeSourceUnits = units
	cache.recentVisualforce = generation.recentVisualforce
	// Capture just observed every Visualforce input, so the first boundary
	// check does not stat them again.
	cache.visualforceInputs = generation.visualforceInputs
	cache.vfVerified, cache.vfEpoch = len(generation.visualforceInputs) > 0, snapshotValidationEpoch.Load()
	if cache.capturedMetadata == nil {
		cache.capturedMetadata = make(map[string]typesys.ApexMetadataInput)
	}
	for path := range generation.inputs {
		cache.capturedMetadata[path] = generation.inputs[path]
	}
	return nil
}

// captureVisualforceRuntimeInputs includes every project file used by page
// validation in the runtime identity. Projects without pages keep their
// existing key. The same view observes every file for debugger builds.
func captureVisualforceRuntimeInputs(p project.Project, generation *runtimeMetadataGeneration) error {
	var projects []project.Project
	seen := map[string]bool{}
	var visit func(project.Project)
	visit = func(q project.Project) {
		if seen[q.Root] {
			return
		}
		seen[q.Root] = true
		projects = append(projects, q)
		for _, dep := range q.ManagedPackageDependencies {
			if dep.Status == "loaded" && dep.Project != nil {
				visit(*dep.Project)
			}
		}
	}
	visit(p)
	hasPages := false
	for _, q := range projects {
		hasPages = hasPages || len(q.VisualforcePageFiles) != 0
	}
	if !hasPages {
		return nil
	}
	var diskInputs []string
	useDiskDigests := generation.allowVisualforceDiskDigests && p.Inputs() == nil && p.Root != "" && diskCacheEnabled() && firstVisualforceDigestGeneration(p.Root)
	generation.persistVisualforceAtEnd = useDiskDigests
	if useDiskDigests {
		for _, q := range projects {
			if q.Inputs() != nil {
				continue
			}
			for group, paths := range visualforceRuntimeInputGroups(q) {
				for _, path := range paths {
					diskInputs = append(diskInputs, path)
					if group < 2 && !strings.HasSuffix(strings.ToLower(path), "-meta.xml") {
						diskInputs = append(diskInputs, path+"-meta.xml")
					}
				}
			}
		}
		_ = loadVisualforceDigestCache(p.Root, diskInputs)
	}
	for _, q := range projects {
		groups := visualforceRuntimeInputGroups(q)
		for group, paths := range groups {
			for _, path := range paths {
				inputs := []string{path}
				if group < 2 && !strings.HasSuffix(strings.ToLower(path), "-meta.xml") {
					inputs = append(inputs, path+"-meta.xml")
				}
				for _, input := range inputs {
					if q.Inputs() == nil {
						if generation.visualforceInputs == nil {
							generation.visualforceInputs = make(map[string]bool)
						}
						generation.visualforceInputs[input] = true
					}
					if _, ok := generation.inputs[input]; ok {
						continue
					}
					var digest [sha256.Size]byte
					var err error
					if q.Inputs() == nil {
						if trusted, ok := oldTrustedVisualforceDigest(input); ok {
							generation.inputs[input] = typesys.ApexMetadataInput{Present: true, Digest: trusted}
							continue
						}
						recent, captured, captureErr := captureRecentVisualforceInput(input, recentVisualforceBudget-generation.recentVisualforceBytes)
						if captured {
							err = captureErr
							if err == nil {
								digest = recent.digest
								if generation.recentVisualforce == nil {
									generation.recentVisualforce = make(map[string]recentVisualforceInput)
								}
								generation.recentVisualforce[input] = recent
								generation.recentVisualforceBytes += int64(len(recent.content))
							}
						} else {
							digest, err = currentFileDigest(input)
						}
					} else {
						var data []byte
						data, err = q.ReadFile(input)
						if err == nil {
							digest = sha256.Sum256(data)
						}
					}
					if os.IsNotExist(err) {
						generation.inputs[input] = typesys.ApexMetadataInput{}
						continue
					}
					if err != nil {
						return err
					}
					generation.inputs[input] = typesys.ApexMetadataInput{Present: true, Digest: digest}
				}
			}
		}
	}
	if useDiskDigests {
		_ = saveVisualforceDigestCache(p.Root, diskInputs)
	}
	return nil
}

func visualforceRuntimeInputGroups(q project.Project) [][]string {
	return [][]string{q.VisualforcePageFiles, q.VisualforceComponentFiles,
		q.EmailTemplateFiles, q.FlowFiles, q.LabelFiles, q.TranslationFiles,
		q.StaticResourceFiles, q.StaticResourceMetas, q.MessageChannelFiles,
		q.TabFiles, q.ProfileFiles, q.PermissionSetFiles, q.PermissionSetGroupFiles}
}

// dependencyAdminProfileFiles lists the System Administrator profile files of
// loaded managed package dependencies; record type availability merges them
// with the project's own Admin profile.
func dependencyAdminProfileFiles(p project.Project) []string {
	seen := make(map[*project.Project]bool)
	var files []string
	var visit func(*project.Project)
	visit = func(proj *project.Project) {
		for _, dep := range proj.ManagedPackageDependencies {
			if dep.Status != "loaded" || dep.Project == nil || seen[dep.Project] {
				continue
			}
			seen[dep.Project] = true
			for _, file := range dep.Project.ProfileFiles {
				if profileNameFromPath(file) == "System Administrator" {
					files = append(files, file)
				}
			}
			visit(dep.Project)
		}
	}
	visit(&p)
	return files
}

// captureProviderAdapterGeneration derives adapter paths from the same metadata
// bytes stored in the generation, never a second read of a changing file.
func captureProviderAdapterGeneration(p project.Project, paths []string, bodies map[string]string, generation *runtimeMetadataGeneration, read func(string) ([]byte, error)) error {
	capture := func(path string) error {
		if _, exists := bodies[path]; exists {
			return nil
		}
		data, err := read(path)
		if err != nil {
			return err
		}
		bodies[path] = string(data)
		generation.inputs[path] = typesys.ApexMetadataInput{Present: true, Digest: sha256.Sum256(data)}
		return nil
	}
	for _, path := range paths {
		if err := capture(path); err != nil {
			return err
		}
	}
	adapterPaths, err := resource.PaymentGatewayProviderSourceFiles(p, func(path string) ([]byte, error) {
		if err := capture(path); err != nil {
			return nil, err
		}
		body, ok := bodies[path]
		if !ok {
			return nil, fmt.Errorf("provider metadata %q missing from snapshot", path)
		}
		return []byte(body), nil
	})
	if err != nil {
		return err
	}
	for _, path := range adapterPaths {
		if err := capture(path); err != nil {
			return err
		}
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

// readSourceRootFile reads through read, which is a project input view reader
// when the caller has a loaded project.
func readSourceRootFile(read func(string) ([]byte, error), root, path string) ([]byte, error) {
	path, err := sourceRootPath(root, path)
	if err != nil {
		return nil, err
	}
	return read(filepath.Clean(path))
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
	return readSourceRootFile(snapshot.ReadFile, root, path)
}
