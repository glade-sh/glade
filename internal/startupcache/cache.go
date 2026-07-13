package startupcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const Version = 4

const manifestSchemaVersion = 1

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
	PlatformABI string           `json:"platformAbi"`
	RuntimeABI  string           `json:"runtimeAbi,omitempty"`
	Manifest    Manifest         `json:"manifest"`
	Org         storage.OrgState `json:"org"`
	Runtime     CompiledRuntime  `json:"runtime"`
}

type Manifest struct {
	SchemaVersion    int         `json:"schemaVersion"`
	ProjectRoot      string      `json:"projectRoot"`
	SourceAPIVersion string      `json:"sourceApiVersion,omitempty"`
	Namespace        string      `json:"namespace,omitempty"`
	ProjectDigest    string      `json:"projectDigest"`
	Features         []string    `json:"features,omitempty"`
	Complete         bool        `json:"complete"`
	Files            []File      `json:"files"`
	ConfigFiles      []File      `json:"configFiles"`
	PackageRoots     []Directory `json:"packageRoots"`
}

type File struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
	SHA256  string `json:"sha256"`
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
	return freshManifest(entry.Version, entry.ProjectRoot, entry.PlatformABI, entry.Manifest, projectRoot, expectedVersion)
}

func FreshRuntime(entry *Entry, projectRoot string, expectedVersion int, expectedRuntimeABI string) bool {
	if !Fresh(entry, projectRoot, expectedVersion) {
		return false
	}
	return entry.RuntimeABI != "" && entry.RuntimeABI == expectedRuntimeABI
}

func freshManifest(version int, entryProjectRoot, entryPlatformABI string, manifest Manifest, projectRoot string, expectedVersion int) bool {
	if version != expectedVersion || entryPlatformABI == "" || entryPlatformABI != currentPlatformABI() {
		return false
	}
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return false
	}
	root = filepath.Clean(root)
	if filepath.Clean(entryProjectRoot) != root {
		return false
	}
	if filepath.Clean(manifest.ProjectRoot) != root {
		return false
	}
	if manifest.SchemaVersion != manifestSchemaVersion || !manifest.Complete || manifest.ProjectDigest == "" {
		return false
	}
	loaded, err := project.Load(root)
	if err != nil {
		return false
	}
	current, err := buildManifest(root, loaded, false)
	if err != nil || !current.Complete {
		return false
	}
	if manifest.SourceAPIVersion != current.SourceAPIVersion ||
		manifest.Namespace != current.Namespace ||
		manifest.ProjectDigest != current.ProjectDigest ||
		!reflect.DeepEqual(manifest.Features, current.Features) ||
		!sameFileMetadataSet(manifest.Files, current.Files) ||
		!sameFileMetadataSet(manifest.ConfigFiles, current.ConfigFiles) ||
		!sameDirectorySet(manifest.PackageRoots, current.PackageRoots) {
		return false
	}
	for _, fp := range append(append([]File(nil), manifest.Files...), manifest.ConfigFiles...) {
		if !fileFingerprintMatches(root, fp, true) {
			return false
		}
	}
	return true
}

func BuildManifest(projectRoot string, p project.Project, index typesys.Index) Manifest {
	_ = index
	manifest, _ := buildManifest(projectRoot, p, true)
	return manifest
}

func buildManifest(projectRoot string, p project.Project, includeHashes bool) (Manifest, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return Manifest{}, err
	}
	root = filepath.Clean(root)
	paths := make([]File, 0, 128)
	for _, manifestProject := range manifestProjects(p) {
		for _, file := range deduplicateStrings(appendProjectFilesFromProject(manifestProject)) {
			abs := absoluteProjectPath(manifestProject.Root, file)
			fp, err := fingerprintFile(root, abs, includeHashes)
			if err != nil {
				return incompleteManifest(root, p), err
			}
			paths = append(paths, fp)
		}
		for _, source := range manifestProject.ApexFiles {
			sidecar := absoluteProjectPath(manifestProject.Root, source) + "-meta.xml"
			fp, exists, err := fingerprintOptionalFile(root, sidecar, includeHashes)
			if err != nil {
				return incompleteManifest(root, p), err
			}
			if exists {
				paths = append(paths, fp)
			}
		}
	}
	runtimeInputs, err := runtimeDiscoveredInputPaths(p)
	if err != nil {
		return incompleteManifest(root, p), err
	}
	for _, input := range runtimeInputs {
		fp, err := fingerprintFile(root, input, includeHashes)
		if err != nil {
			return incompleteManifest(root, p), err
		}
		paths = append(paths, fp)
	}
	for _, artifactPath := range dependencyArtifactPaths(p) {
		fp, err := fingerprintFile(root, artifactPath, includeHashes)
		if err != nil {
			return incompleteManifest(root, p), err
		}
		paths = append(paths, fp)
	}
	configPaths := make([]File, 0, 16)
	for _, manifestProject := range manifestProjects(p) {
		candidates, err := collectConfigPaths(manifestProject)
		if err != nil {
			return incompleteManifest(root, p), err
		}
		for _, configPath := range candidates {
			fp, exists, err := fingerprintOptionalFile(root, configPath, includeHashes)
			if err != nil {
				return incompleteManifest(root, p), err
			}
			if exists {
				configPaths = append(configPaths, fp)
			}
		}
	}
	packageRoots := make([]Directory, 0, len(p.PackageDirectories))
	for _, manifestProject := range manifestProjects(p) {
		for _, pkgRoot := range packageRootsForProject(manifestProject) {
			fp, ok, err := statOptionalDirectory(root, pkgRoot)
			if err != nil {
				return incompleteManifest(root, p), err
			}
			if ok {
				packageRoots = append(packageRoots, fp)
			}
		}
	}
	paths = deduplicateFileFingerprints(paths)
	configPaths = deduplicateFileFingerprints(configPaths)
	sort.Slice(packageRoots, func(i, j int) bool { return packageRoots[i].Path < packageRoots[j].Path })
	features := append([]string(nil), project.OrgShapeFeatures(root)...)
	sort.Strings(features)
	digest, err := semanticProjectDigest(root, p, features)
	if err != nil {
		return incompleteManifest(root, p), err
	}
	return Manifest{
		SchemaVersion:    manifestSchemaVersion,
		ProjectRoot:      root,
		SourceAPIVersion: p.SourceAPIVersion,
		Namespace:        p.Namespace,
		ProjectDigest:    digest,
		Features:         features,
		Complete:         true,
		Files:            paths,
		ConfigFiles:      configPaths,
		PackageRoots:     deduplicateDirectoryFingerprints(packageRoots),
	}, nil
}

func absoluteProjectPath(root, path string) string {
	abs := filepath.Clean(path)
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, abs)
	}
	return abs
}

func runtimeDiscoveredInputPaths(p project.Project) ([]string, error) {
	if strings.TrimSpace(p.Root) == "" {
		return nil, nil
	}
	notificationTypes, err := notificationTypeInputPaths(p.Root)
	if err != nil {
		return nil, err
	}
	projectData, err := projectDataInputPaths(p.Root)
	if err != nil {
		return nil, err
	}
	paths := append(notificationTypes, projectData...)
	for _, dependency := range p.ManagedPackageDependencies {
		if dependency.Project == nil || dependency.Status != "loaded" {
			continue
		}
		dependencyData, err := projectDataInputPaths(dependency.Project.Root)
		if err != nil {
			return nil, err
		}
		paths = append(paths, dependencyData...)
	}
	return deduplicateStrings(paths), nil
}

func notificationTypeInputPaths(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		lower := strings.ToLower(filepath.ToSlash(path))
		if strings.HasSuffix(lower, ".notiftype-meta.xml") || strings.HasSuffix(lower, ".notiftype") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

func projectDataInputPaths(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root && entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") || skipProjectDataInputDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		if strings.Contains(filepath.ToSlash(path), "/data/") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

func skipProjectDataInputDir(name string) bool {
	switch strings.ToLower(name) {
	case "node_modules", "vendor", "dist", "bin", "coverage", "__tests__", "__mocks__":
		return true
	default:
		return false
	}
}

func NewEntry(projectRoot string, p project.Project, index typesys.Index, org storage.OrgState, runtime CompiledRuntime) Entry {
	root, _ := filepath.Abs(projectRoot)
	root = filepath.Clean(root)
	return Entry{
		Version:     Version,
		ProjectRoot: root,
		BuiltAt:     time.Now().Format(time.RFC3339Nano),
		PlatformABI: currentPlatformABI(),
		Manifest:    BuildManifest(root, p, index),
		Org:         org,
		Runtime:     runtime,
	}
}

func fileFingerprintMatches(projectRoot string, expected File, required bool) bool {
	absPath := filepath.Clean(filepath.Join(projectRoot, filepath.FromSlash(expected.Path)))
	file, err := os.Open(absPath)
	if err != nil {
		return !required && errors.Is(err, os.ErrNotExist)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return false
	}
	if info.IsDir() || info.Size() != expected.Size || info.ModTime().UnixNano() != expected.ModTime {
		return false
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}
	after, err := file.Stat()
	if err != nil || after.Size() != info.Size() || after.ModTime() != info.ModTime() {
		return false
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	return len(expected.SHA256) == sha256.Size*2 && got == expected.SHA256
}

func statFile(projectRoot, absPath string) (File, bool) {
	fp, err := fingerprintFile(projectRoot, absPath, true)
	if err != nil {
		return File{}, false
	}
	return fp, true
}

func fingerprintFile(projectRoot, path string, includeHash bool) (File, error) {
	file, err := os.Open(path)
	if err != nil {
		return File{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return File{}, err
	}
	if info.IsDir() {
		return File{}, errors.New("startup cache input is a directory")
	}
	rel, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return File{}, err
	}
	fp := File{
		Path:    filepath.ToSlash(filepath.Clean(rel)),
		Size:    info.Size(),
		ModTime: info.ModTime().UnixNano(),
	}
	if !includeHash {
		return fp, nil
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return File{}, err
	}
	after, err := file.Stat()
	if err != nil || after.Size() != info.Size() || after.ModTime() != info.ModTime() {
		return File{}, errors.New("startup cache input changed while hashing")
	}
	fp.SHA256 = hex.EncodeToString(hasher.Sum(nil))
	return fp, nil
}

func currentPlatformABI() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func incompleteManifest(root string, p project.Project) Manifest {
	return Manifest{
		SchemaVersion:    manifestSchemaVersion,
		ProjectRoot:      root,
		SourceAPIVersion: p.SourceAPIVersion,
		Namespace:        p.Namespace,
	}
}

func fingerprintOptionalFile(projectRoot, path string, includeHash bool) (File, bool, error) {
	fp, err := fingerprintFile(projectRoot, path, includeHash)
	if errors.Is(err, os.ErrNotExist) {
		return File{}, false, nil
	}
	if err != nil {
		return File{}, false, err
	}
	return fp, true, nil
}

func statOptionalDirectory(projectRoot, path string) (Directory, bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return Directory{}, false, nil
	}
	if err != nil {
		return Directory{}, false, err
	}
	if !info.IsDir() {
		return Directory{}, false, errors.New("startup cache package root is not a directory")
	}
	rel, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return Directory{}, false, err
	}
	return Directory{
		Path:    filepath.ToSlash(filepath.Clean(rel)),
		ModTime: info.ModTime().UnixNano(),
	}, true, nil
}

func sameFileMetadataSet(expected, current []File) bool {
	if len(expected) != len(current) {
		return false
	}
	for i := range expected {
		if expected[i].Path == "" ||
			expected[i].Path != current[i].Path ||
			expected[i].Size != current[i].Size ||
			expected[i].ModTime != current[i].ModTime ||
			len(expected[i].SHA256) != sha256.Size*2 {
			return false
		}
		if i > 0 && expected[i-1].Path >= expected[i].Path {
			return false
		}
	}
	return true
}

func sameDirectorySet(expected, current []Directory) bool {
	if len(expected) != len(current) {
		return false
	}
	for i := range expected {
		if expected[i].Path == "" || expected[i].Path != current[i].Path {
			return false
		}
		if i > 0 && expected[i-1].Path >= expected[i].Path {
			return false
		}
	}
	return true
}

func deduplicateFileFingerprints(values []File) []File {
	if len(values) == 0 {
		return nil
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Path < values[j].Path })
	out := values[:0]
	for _, value := range values {
		if value.Path == "" || (len(out) > 0 && out[len(out)-1].Path == value.Path) {
			continue
		}
		out = append(out, value)
	}
	return out
}

type semanticProject struct {
	Root               string                     `json:"root"`
	Namespace          string                     `json:"namespace,omitempty"`
	SourceAPIVersion   string                     `json:"sourceApiVersion,omitempty"`
	PackageDirectories []project.PackageDirectory `json:"packageDirectories"`
	NamespaceRemaps    []string                   `json:"namespaceRemaps,omitempty"`
	Dependencies       []semanticDependency       `json:"dependencies,omitempty"`
	PackageShims       []semanticPackageShim      `json:"packageShims,omitempty"`
}

type semanticDependency struct {
	Namespace    string           `json:"namespace"`
	SourceRoot   string           `json:"sourceRoot,omitempty"`
	ArtifactPath string           `json:"artifactPath,omitempty"`
	Version      string           `json:"version,omitempty"`
	Status       string           `json:"status"`
	Project      *semanticProject `json:"project,omitempty"`
}

type semanticPackageShim struct {
	Namespace  string           `json:"namespace"`
	SourceRoot string           `json:"sourceRoot"`
	Status     string           `json:"status"`
	Project    *semanticProject `json:"project,omitempty"`
}

type semanticManifest struct {
	Project  semanticProject `json:"project"`
	Features []string        `json:"features,omitempty"`
}

func semanticProjectDigest(root string, p project.Project, features []string) (string, error) {
	semantic := semanticManifest{
		Project:  projectSemantics(root, p, make(map[string]bool)),
		Features: append([]string(nil), features...),
	}
	data, err := json.Marshal(semantic)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func projectSemantics(root string, p project.Project, seen map[string]bool) semanticProject {
	projectRoot := filepath.Clean(p.Root)
	out := semanticProject{
		Root:               semanticPath(root, projectRoot),
		Namespace:          p.Namespace,
		SourceAPIVersion:   p.SourceAPIVersion,
		PackageDirectories: canonicalPackageDirectories(p.PackageDirectories),
	}
	for _, remap := range p.NamespaceRemaps {
		out.NamespaceRemaps = append(out.NamespaceRemaps, remap.From+":"+remap.To)
	}
	sort.Strings(out.NamespaceRemaps)
	if seen[projectRoot] {
		return out
	}
	seen[projectRoot] = true
	defer delete(seen, projectRoot)
	for _, dep := range p.ManagedPackageDependencies {
		semantic := semanticDependency{
			Namespace:    dep.Namespace,
			SourceRoot:   semanticPath(root, dep.SourceRoot),
			ArtifactPath: semanticPath(root, dep.ArtifactPath),
			Version:      dep.Version,
			Status:       dep.Status,
		}
		if dep.Project != nil {
			child := projectSemantics(root, *dep.Project, seen)
			semantic.Project = &child
		}
		out.Dependencies = append(out.Dependencies, semantic)
	}
	sort.Slice(out.Dependencies, func(i, j int) bool {
		left, right := out.Dependencies[i], out.Dependencies[j]
		return strings.Join([]string{left.Namespace, left.SourceRoot, left.ArtifactPath, left.Version, left.Status}, "\x00") <
			strings.Join([]string{right.Namespace, right.SourceRoot, right.ArtifactPath, right.Version, right.Status}, "\x00")
	})
	for _, shim := range p.PackageShims {
		semantic := semanticPackageShim{
			Namespace:  shim.Namespace,
			SourceRoot: semanticPath(root, shim.SourceRoot),
			Status:     shim.Status,
		}
		if shim.Project != nil {
			child := projectSemantics(root, *shim.Project, seen)
			semantic.Project = &child
		}
		out.PackageShims = append(out.PackageShims, semantic)
	}
	sort.Slice(out.PackageShims, func(i, j int) bool {
		left, right := out.PackageShims[i], out.PackageShims[j]
		return strings.Join([]string{left.Namespace, left.SourceRoot, left.Status}, "\x00") <
			strings.Join([]string{right.Namespace, right.SourceRoot, right.Status}, "\x00")
	})
	return out
}

func canonicalPackageDirectories(values []project.PackageDirectory) []project.PackageDirectory {
	out := make([]project.PackageDirectory, len(values))
	for i, value := range values {
		out[i] = value
		out[i].Path = filepath.ToSlash(filepath.Clean(value.Path))
		out[i].Dependencies = append([]project.PackageDependency(nil), value.Dependencies...)
		sort.Slice(out[i].Dependencies, func(left, right int) bool {
			leftDep, rightDep := out[i].Dependencies[left], out[i].Dependencies[right]
			return leftDep.Package+"\x00"+leftDep.VersionNumber < rightDep.Package+"\x00"+rightDep.VersionNumber
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Package != out[j].Package {
			return out[i].Package < out[j].Package
		}
		if out[i].Default != out[j].Default {
			return !out[i].Default
		}
		return packageDependencyKey(out[i].Dependencies) < packageDependencyKey(out[j].Dependencies)
	})
	return out
}

func packageDependencyKey(values []project.PackageDependency) string {
	parts := make([]string, 0, len(values)*2)
	for _, value := range values {
		parts = append(parts, value.Package, value.VersionNumber)
	}
	return strings.Join(parts, "\x00")
}

func semanticPath(root, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return filepath.ToSlash(clean)
	}
	rel, err := filepath.Rel(root, clean)
	if err != nil {
		return filepath.ToSlash(clean)
	}
	return filepath.ToSlash(filepath.Clean(rel))
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
	var appendProjects func(project.Project)
	appendProjects = func(parent project.Project) {
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
			appendProjects(*dep.Project)
		}
		for _, shim := range parent.PackageShims {
			if shim.Project == nil || shim.Project.Root == "" {
				continue
			}
			root := filepath.Clean(shim.Project.Root)
			if seen[root] {
				continue
			}
			seen[root] = true
			out = append(out, *shim.Project)
			appendProjects(*shim.Project)
		}
	}
	appendProjects(p)
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

func collectConfigPaths(p project.Project) ([]string, error) {
	root := p.Root
	paths := []string{
		filepath.Join(root, "sfdx-project.json"),
		filepath.Join(root, "glade.yml"),
		filepath.Join(root, "config", "project-scratch-def.json"),
		filepath.Join(root, "config", "hc-project-scratch-def.json"),
		filepath.Join(root, "cumulusci.yml"),
		filepath.Join(root, "cumulusci.template.yml"),
	}
	for dir := filepath.Clean(root); ; dir = filepath.Dir(dir) {
		nearest := filepath.Join(dir, "glade.yml")
		if _, err := os.Stat(nearest); err == nil {
			paths = append(paths, nearest)
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	for _, name := range []string{"cumulusci.yml", "cumulusci.template.yml"} {
		configPath := filepath.Join(root, name)
		data, err := os.ReadFile(configPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "config_file:") {
				continue
			}
			path := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "config_file:")), `"'`)
			if path == "" || filepath.IsAbs(path) || !strings.HasSuffix(strings.ToLower(path), ".json") {
				continue
			}
			full := filepath.Clean(filepath.Join(root, filepath.FromSlash(path)))
			if full == filepath.Clean(root) || !strings.HasPrefix(full, filepath.Clean(root)+string(os.PathSeparator)) {
				continue
			}
			paths = append(paths, full)
		}
	}
	return deduplicateStrings(paths), nil
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
