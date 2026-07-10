package startupcache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const Version = 3

// DAPCacheVersion matches the historical DAP startup cache version.
const DAPCacheVersion = 3

// SubdirTest is the on-disk cache directory used by glade test.
const SubdirTest = ".glade/test"

// SubdirDAP is the on-disk cache used by the DAP/debug path.
const SubdirDAP = ".glade/dap"

const stateFile = "startup.json"

type CompiledRuntime struct {
	Methods   map[string]vm.Method `json:"methods,omitempty"`
	Classes   []vm.Class           `json:"classes,omitempty"`
	Triggers  []vm.Trigger         `json:"triggers,omitempty"`
	PageNames []string             `json:"pageNames,omitempty"`
}

type Entry struct {
	Version     int              `json:"version"`
	ProjectRoot string           `json:"projectRoot"`
	BuiltAt     string           `json:"builtAt"`
	RuntimeABI  string           `json:"runtimeAbi,omitempty"`
	Manifest    Manifest         `json:"manifest"`
	Org         storage.OrgState `json:"org"`
	Runtime     CompiledRuntime  `json:"runtime"`
}

type Manifest struct {
	ProjectRoot      string      `json:"projectRoot"`
	SourceAPIVersion string      `json:"sourceApiVersion,omitempty"`
	Namespace        string      `json:"namespace,omitempty"`
	Files            []File      `json:"files"`
	ConfigFiles      []File      `json:"configFiles"`
	PackageRoots     []Directory `json:"packageRoots"`
}

type File struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

type Directory struct {
	Path    string `json:"path"`
	ModTime int64  `json:"modTime"`
}

type ReadStats struct {
	ValidationNS int64 `json:"validationNs,omitempty"`
	DecodeNS     int64 `json:"decodeNs,omitempty"`
}

type WriteStats struct {
	EncodeNS int64 `json:"encodeNs,omitempty"`
}

func cachePath(projectRoot, subdir string) string {
	return filepath.Join(projectRoot, filepath.FromSlash(subdir), stateFile)
}

func Read(projectRoot, subdir string) (*Entry, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	if subdir == SubdirTest {
		return readGob(root, subdir)
	}
	return readJSON(root, subdir)
}

func ReadWithStats(projectRoot, subdir string) (*Entry, ReadStats, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, ReadStats{}, err
	}
	root = filepath.Clean(root)
	if subdir == SubdirTest {
		return readSplitTestCacheWithStats(root, subdir)
	}
	entry, err := readJSON(root, subdir)
	return entry, ReadStats{}, err
}

func readJSON(projectRoot, subdir string) (*Entry, error) {
	data, err := os.ReadFile(cachePath(projectRoot, subdir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, nil
	}
	return &entry, nil
}

func Write(entry *Entry, subdir string) error {
	if subdir == SubdirTest {
		return writeGob(entry, subdir)
	}
	return writeJSON(entry, subdir)
}

func WriteWithStats(entry *Entry, subdir string) (WriteStats, error) {
	if subdir == SubdirTest {
		return writeSplitTestCacheWithStats(entry, subdir)
	}
	return WriteStats{}, writeJSON(entry, subdir)
}

func writeJSON(entry *Entry, subdir string) error {
	if entry == nil || entry.ProjectRoot == "" {
		return errors.New("startup cache entry requires project root")
	}
	cacheDir := filepath.Join(entry.ProjectRoot, filepath.FromSlash(subdir))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(cacheDir, stateFile)
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
		if err := enc.Encode(entry); err != nil {
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

func Fresh(entry *Entry, projectRoot string, expectedVersion int) bool {
	if entry == nil {
		return false
	}
	return freshManifest(entry.Version, entry.ProjectRoot, entry.Manifest, projectRoot, expectedVersion)
}

func FreshRuntime(entry *Entry, projectRoot string, expectedVersion int, expectedRuntimeABI string) bool {
	if !Fresh(entry, projectRoot, expectedVersion) {
		return false
	}
	return entry.RuntimeABI != "" && entry.RuntimeABI == expectedRuntimeABI
}

func freshManifest(version int, entryProjectRoot string, manifest Manifest, projectRoot string, expectedVersion int) bool {
	if version != expectedVersion {
		return false
	}
	root := filepath.Clean(projectRoot)
	if filepath.Clean(entryProjectRoot) != root {
		return false
	}
	if filepath.Clean(manifest.ProjectRoot) != root {
		return false
	}
	for _, fp := range manifest.Files {
		if !fileFingerprintMatches(root, fp, true) {
			return false
		}
	}
	for _, fp := range manifest.ConfigFiles {
		if !fileFingerprintMatches(root, fp, true) {
			return false
		}
	}
	for _, dir := range manifest.PackageRoots {
		if !dirFingerprintMatches(root, dir) {
			return false
		}
	}
	return true
}

func BuildManifest(projectRoot string, p project.Project, index typesys.Index) Manifest {
	paths := make([]File, 0, 128)
	for _, manifestProject := range manifestProjects(p) {
		for _, file := range deduplicateStrings(appendProjectFilesFromProject(manifestProject)) {
			abs := filepath.Clean(file)
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(manifestProject.Root, abs)
			}
			if fp, ok := statFile(projectRoot, abs); ok {
				paths = append(paths, fp)
			}
		}
	}
	for _, artifactPath := range dependencyArtifactPaths(p) {
		if fp, ok := statFile(projectRoot, artifactPath); ok {
			paths = append(paths, fp)
		}
	}
	configPaths := make([]File, 0, 16)
	for _, manifestProject := range manifestProjects(p) {
		for _, configPath := range collectConfigPaths(manifestProject) {
			if fp, ok := statFile(projectRoot, configPath); ok {
				configPaths = append(configPaths, fp)
			}
		}
	}
	packageRoots := make([]Directory, 0, len(p.PackageDirectories))
	for _, manifestProject := range manifestProjects(p) {
		for _, pkgRoot := range packageRootsForProject(manifestProject) {
			if fp, ok := statDirectory(projectRoot, pkgRoot); ok {
				packageRoots = append(packageRoots, fp)
			}
		}
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].Path < paths[j].Path })
	sort.Slice(configPaths, func(i, j int) bool { return configPaths[i].Path < configPaths[j].Path })
	sort.Slice(packageRoots, func(i, j int) bool { return packageRoots[i].Path < packageRoots[j].Path })
	return Manifest{
		ProjectRoot:      projectRoot,
		SourceAPIVersion: index.Project.SourceAPIVersion,
		Namespace:        index.Project.Namespace,
		Files:            paths,
		ConfigFiles:      configPaths,
		PackageRoots:     deduplicateDirectoryFingerprints(packageRoots),
	}
}

func NewEntry(projectRoot string, p project.Project, index typesys.Index, org storage.OrgState, runtime CompiledRuntime) Entry {
	root, _ := filepath.Abs(projectRoot)
	root = filepath.Clean(root)
	return Entry{
		Version:     Version,
		ProjectRoot: root,
		BuiltAt:     time.Now().Format(time.RFC3339Nano),
		Manifest:    BuildManifest(root, p, index),
		Org:         org,
		Runtime:     runtime,
	}
}

func fileFingerprintMatches(projectRoot string, expected File, required bool) bool {
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

func dirFingerprintMatches(projectRoot string, expected Directory) bool {
	absPath := filepath.Clean(filepath.Join(projectRoot, filepath.FromSlash(expected.Path)))
	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() || info.ModTime().UnixNano() != expected.ModTime {
		return false
	}
	return true
}

func statFile(projectRoot, absPath string) (File, bool) {
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		return File{}, false
	}
	path, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		return File{}, false
	}
	return File{
		Path:    filepath.ToSlash(filepath.Clean(path)),
		Size:    info.Size(),
		ModTime: info.ModTime().UnixNano(),
	}, true
}

func statDirectory(projectRoot, absPath string) (Directory, bool) {
	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		return Directory{}, false
	}
	path, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		return Directory{}, false
	}
	return Directory{
		Path:    filepath.ToSlash(filepath.Clean(path)),
		ModTime: info.ModTime().UnixNano(),
	}, true
}

func appendProjectFilesFromProject(p project.Project) []string {
	out := make([]string, 0, 128)
	pValue := reflect.ValueOf(p)
	for i := 0; i < pValue.NumField(); i++ {
		field := pValue.Field(i)
		if field.Kind() != reflect.Slice || field.Type().Elem().Kind() != reflect.String {
			continue
		}
		for _, value := range field.Interface().([]string) {
			if value != "" {
				out = append(out, filepath.Clean(value))
			}
		}
	}
	return out
}

func manifestProjects(p project.Project) []project.Project {
	out := []project.Project{p}
	seen := map[string]bool{filepath.Clean(p.Root): true}
	var appendDeps func(project.Project)
	appendDeps = func(parent project.Project) {
		for _, dep := range parent.ManagedPackageDependencies {
			if dep.Project == nil || dep.Project.Root == "" {
				continue
			}
			root := filepath.Clean(dep.Project.Root)
			if seen[root] {
				continue
			}
			seen[root] = true
			out = append(out, *dep.Project)
			appendDeps(*dep.Project)
		}
	}
	appendDeps(p)
	return out
}

func dependencyArtifactPaths(p project.Project) []string {
	var out []string
	var appendDeps func(project.Project)
	appendDeps = func(parent project.Project) {
		for _, dep := range parent.ManagedPackageDependencies {
			if dep.ArtifactPath != "" {
				out = append(out, filepath.Clean(dep.ArtifactPath))
			}
			if dep.Project != nil {
				appendDeps(*dep.Project)
			}
		}
	}
	appendDeps(p)
	return deduplicateStrings(out)
}

func collectConfigPaths(p project.Project) []string {
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

func deduplicateDirectoryFingerprints(values []Directory) []Directory {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]Directory, 0, len(values))
	for _, value := range values {
		if value.Path == "" {
			continue
		}
		key := filepath.Clean(filepath.ToSlash(value.Path))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, Directory{Path: key, ModTime: value.ModTime})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
