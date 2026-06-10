package gladecli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

const (
	dapCacheVersion   = 3
	dapCacheDirName   = ".glade/dap"
	dapCacheStateFile = "startup.json"
)

type dapStartupCache struct {
	Version     int                             `json:"version"`
	ProjectRoot string                          `json:"projectRoot"`
	BuiltAt     string                          `json:"builtAt"`
	Manifest    dapStartupManifest              `json:"manifest"`
	Org         storage.OrgState                `json:"org"`
	Runtime     apextest.CompiledProjectRuntime `json:"runtime"`
}

type dapStartupManifest struct {
	ProjectRoot      string              `json:"projectRoot"`
	SourceAPIVersion string              `json:"sourceApiVersion,omitempty"`
	Namespace        string              `json:"namespace,omitempty"`
	Files            []dapCacheFile      `json:"files"`
	ConfigFiles      []dapCacheFile      `json:"configFiles"`
	PackageRoots     []dapCacheDirectory `json:"packageRoots"`
}

type dapCacheFile struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

type dapCacheDirectory struct {
	Path    string `json:"path"`
	ModTime int64  `json:"modTime"`
}

func loadDAPStartupState(projectRoot string) (storage.OrgState, apextest.CompiledProjectRuntime, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, err
	}
	root = filepath.Clean(root)
	cache, err := readDapStartupCache(root)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, err
	}
	if cache != nil && isDapStartupCacheFresh(cache, root) {
		return cache.Org, cache.Runtime, nil
	}
	p, index, err := loadProjectIndex(root)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, err
	}
	org := orgStateFromIndex(root, p, index)
	runtime := apextest.CompileProjectRuntimeForRequest(index)
	built := buildDAPStartupCache(root, p, index, org, runtime)
	if err := writeDapStartupCache(&built); err != nil {
		// Caching is an optimization; keep serving from rebuilt state even if
		// cache persistence fails.
	}
	return built.Org, built.Runtime, nil
}

func isDapStartupCacheFresh(cache *dapStartupCache, projectRoot string) bool {
	if cache == nil {
		return false
	}
	if cache.Version != dapCacheVersion {
		return false
	}
	if filepath.Clean(cache.ProjectRoot) != projectRoot {
		return false
	}
	manifest := cache.Manifest
	if manifest.ProjectRoot != projectRoot {
		return false
	}
	if len(manifest.Files) > 0 {
		for _, fp := range manifest.Files {
			if !fileFingerprintMatches(projectRoot, fp, false) {
				return false
			}
		}
	}
	for _, fp := range manifest.ConfigFiles {
		if !fileFingerprintMatches(projectRoot, fp, true) {
			return false
		}
	}
	for _, dir := range manifest.PackageRoots {
		if !dirFingerprintMatches(projectRoot, dir) {
			return false
		}
	}
	return true
}

func fileFingerprintMatches(projectRoot string, expected dapCacheFile, required bool) bool {
	absPath := filepath.Clean(filepath.Join(projectRoot, filepath.FromSlash(expected.Path)))
	info, err := os.Stat(absPath)
	if err != nil {
		return !required && errors.Is(err, os.ErrNotExist)
	}
	if info.IsDir() || info.Size() != expected.Size || info.ModTime().UnixNano() != expected.ModTime {
		return false
	}
	return true
}

func dirFingerprintMatches(projectRoot string, expected dapCacheDirectory) bool {
	absPath := filepath.Clean(filepath.Join(projectRoot, filepath.FromSlash(expected.Path)))
	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() || info.ModTime().UnixNano() != expected.ModTime {
		return false
	}
	return true
}

func buildDAPStartupCache(projectRoot string, p project.Project, index typesys.Index, org storage.OrgState, runtime apextest.CompiledProjectRuntime) dapStartupCache {
	paths := make([]dapCacheFile, 0, 128)
	for _, file := range deduplicateStrings(appendProjectFilesFromProject(p)) {
		abs := filepath.Clean(file)
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(projectRoot, abs)
		}
		if fp, ok := statDapFile(projectRoot, abs); ok {
			paths = append(paths, fp)
		}
	}
	configPaths := make([]dapCacheFile, 0, 16)
	for _, configPath := range collectDAPConfigPaths(p) {
		if fp, ok := statDapFile(projectRoot, configPath); ok {
			configPaths = append(configPaths, fp)
		}
	}
	packageRoots := make([]dapCacheDirectory, 0, len(p.PackageDirectories))
	for _, pkgRoot := range packageRootsForProject(p) {
		if fp, ok := statDapDirectory(projectRoot, pkgRoot); ok {
			packageRoots = append(packageRoots, fp)
		}
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].Path < paths[j].Path })
	sort.Slice(configPaths, func(i, j int) bool { return configPaths[i].Path < configPaths[j].Path })
	sort.Slice(packageRoots, func(i, j int) bool { return packageRoots[i].Path < packageRoots[j].Path })

	return dapStartupCache{
		Version:     dapCacheVersion,
		ProjectRoot: projectRoot,
		BuiltAt:     time.Now().Format(time.RFC3339Nano),
		Manifest: dapStartupManifest{
			ProjectRoot:      projectRoot,
			SourceAPIVersion: index.Project.SourceAPIVersion,
			Namespace:        index.Project.Namespace,
			Files:            paths,
			ConfigFiles:      configPaths,
			PackageRoots:     deduplicateDirectoryFingerprints(packageRoots),
		},
		Org:     org,
		Runtime: runtime,
	}
}

func readDapStartupCache(projectRoot string) (*dapStartupCache, error) {
	path := dapStartupCachePath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cache dapStartupCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, nil
	}
	return &cache, nil
}

func writeDapStartupCache(cache *dapStartupCache) error {
	cacheDir := filepath.Join(cache.ProjectRoot, filepath.FromSlash(dapCacheDirName))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(cacheDir, dapCacheStateFile)
	tmp, err := os.CreateTemp(cacheDir, "startup-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	writeErr := func() error {
		defer os.Remove(tmpPath)
		defer tmp.Close()
		enc := json.NewEncoder(tmp)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cache); err != nil {
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		return os.Rename(tmpPath, path)
	}()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return writeErr
	}
	return nil
}

func dapStartupCachePath(projectRoot string) string {
	return filepath.Join(projectRoot, filepath.FromSlash(dapCacheDirName), dapCacheStateFile)
}

func statDapFile(projectRoot, absPath string) (dapCacheFile, bool) {
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		return dapCacheFile{}, false
	}
	path, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		return dapCacheFile{}, false
	}
	return dapCacheFile{
		Path:    filepath.ToSlash(filepath.Clean(path)),
		Size:    info.Size(),
		ModTime: info.ModTime().UnixNano(),
	}, true
}

func statDapDirectory(projectRoot, absPath string) (dapCacheDirectory, bool) {
	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		return dapCacheDirectory{}, false
	}
	path, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		return dapCacheDirectory{}, false
	}
	return dapCacheDirectory{
		Path:    filepath.ToSlash(filepath.Clean(path)),
		ModTime: info.ModTime().UnixNano(),
	}, true
}

func appendProjectFilesFromProject(p project.Project) []string {
	out := make([]string, 0, 128)
	pValue := reflect.ValueOf(p)
	for i := 0; i < pValue.NumField(); i++ {
		field := pValue.Field(i)
		if field.Kind() != reflect.Slice {
			continue
		}
		if field.Type().Elem().Kind() != reflect.String {
			continue
		}
		for _, value := range field.Interface().([]string) {
			if value != "" {
				out = append(out, filepath.Clean(value))
			}
		}
	}
	for _, dep := range p.ManagedPackageDependencies {
		if dep.SourceRoot != "" {
			out = append(out, filepath.Clean(dep.SourceRoot))
		}
		if dep.ArtifactPath != "" {
			out = append(out, filepath.Clean(dep.ArtifactPath))
		}
	}
	return out
}

func collectDAPConfigPaths(p project.Project) []string {
	root := p.Root
	return deduplicateStrings([]string{
		filepath.Join(root, "sfdx-project.json"),
		filepath.Join(root, "glade.yml"),
		filepath.Join(root, "config", "project-scratch-def.json"),
		filepath.Join(root, "config", "hc-project-scratch-def.json"),
		filepath.Join(root, "cumulusci.yml"),
		filepath.Join(root, "cumulusci.template.yml"),
	})
}

func packageRootsForProject(p project.Project) []string {
	roots := make([]string, 0, len(p.PackageDirectories))
	for _, dir := range p.PackageDirectories {
		if dir.Path == "" {
			continue
		}
		roots = append(roots, filepath.Clean(filepath.Join(p.Root, filepath.FromSlash(dir.Path))))
	}
	return deduplicateStrings(roots)
}

func deduplicateStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		value = filepath.Clean(filepath.ToSlash(value))
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func deduplicateDirectoryFingerprints(values []dapCacheDirectory) []dapCacheDirectory {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]dapCacheDirectory, 0, len(values))
	for _, value := range values {
		if value.Path == "" {
			continue
		}
		key := filepath.Clean(filepath.ToSlash(value.Path))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, dapCacheDirectory{
			Path:    key,
			ModTime: value.ModTime,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
