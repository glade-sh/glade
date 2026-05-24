package playground

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const maxPlaygroundFileSize = 512 * 1024
const readOnlyManifestFile = ".glade-playground-readonly.json"

type Workspace struct {
	ID          string
	Root        string
	ProjectRoot string
	DataRoot    string
	Managed     bool
	ExampleID   string

	mu       sync.Mutex
	versions map[string]int
	readOnly map[string]bool
}

func OpenWorkspace(opts WorkspaceOptions) (*Workspace, error) {
	id := strings.TrimSpace(opts.ID)
	if id == "" {
		id = "default"
	}
	dataRoot := opts.DataRoot
	if dataRoot == "" {
		dataRoot = filepath.Join(".glade", "playground")
	}
	root := opts.ProjectRoot
	managed := root == ""
	if root == "" {
		root = filepath.Join(dataRoot, "workspaces", id)
	}
	ws := &Workspace{
		ID:          id,
		Root:        root,
		ProjectRoot: root,
		DataRoot:    dataRoot,
		Managed:     managed,
		versions:    make(map[string]int),
		readOnly:    make(map[string]bool),
	}
	if err := ws.ensureDefaultFiles(); err != nil {
		return nil, err
	}
	if err := ws.loadReadOnlyManifest(); err != nil {
		return nil, err
	}
	if err := ws.refreshVersions(); err != nil {
		return nil, err
	}
	return ws, nil
}

func (w *Workspace) ensureDefaultFiles() error {
	hasFiles := false
	if err := filepath.WalkDir(w.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if path != w.Root && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if isAllowedPlaygroundExtension(filepath.Ext(path)) {
			hasFiles = true
			return fs.SkipAll
		}
		return nil
	}); err != nil {
		return err
	}
	if hasFiles {
		return nil
	}

	defaults := map[string]string{
		"sfdx-project.json": `{"packageDirectories":[{"path":"force-app","default":true}],"name":"glade-playground","namespace":"","sourceApiVersion":"65.0"}` + "\n",
		"force-app/main/default/classes/AccountPlayground.cls": `public class AccountPlayground {
  public static Account makeAccount(String name) {
    Account account = new Account(Name = name);
    insert account;
    return [
      SELECT Id, Name
      FROM Account
      WHERE Id = :account.Id
      LIMIT 1
    ];
  }
}
`,
		"anonymous.apex": "Account account = AccountPlayground.makeAccount('Twin Lakes Supply');\nSystem.debug(account.Name);\n",
		"seed.json":      "{}\n",
	}
	for rel, content := range defaults {
		path := filepath.Join(w.Root, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (w *Workspace) LoadExample(id string) (WorkspaceMetadata, error) {
	example, ok := exampleTemplateByID(id)
	if !ok {
		return WorkspaceMetadata{}, fmt.Errorf("unknown playground example %q", id)
	}
	return w.loadFiles(example.ID, example.Files, false)
}

func (w *Workspace) LoadProjectReference(ref ProjectReference) (WorkspaceMetadata, error) {
	files, err := collectProjectReferenceFiles(ref)
	if err != nil {
		return WorkspaceMetadata{}, err
	}
	return w.loadFiles(ref.ID, files, true)
}

func (w *Workspace) loadFiles(sourceID string, files map[string]string, readOnly bool) (WorkspaceMetadata, error) {
	if !w.Managed {
		return WorkspaceMetadata{}, errors.New("playground projects can only be loaded into the managed scratch workspace")
	}
	w.mu.Lock()
	if strings.TrimSpace(w.Root) == "" {
		w.mu.Unlock()
		return WorkspaceMetadata{}, errors.New("workspace root is required")
	}
	if err := os.RemoveAll(w.Root); err != nil {
		w.mu.Unlock()
		return WorkspaceMetadata{}, err
	}
	w.versions = make(map[string]int)
	w.readOnly = make(map[string]bool)
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		path, err := w.SafePath(rel)
		if err != nil {
			w.mu.Unlock()
			return WorkspaceMetadata{}, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			w.mu.Unlock()
			return WorkspaceMetadata{}, err
		}
		if err := os.WriteFile(path, []byte(files[rel]), 0o644); err != nil {
			w.mu.Unlock()
			return WorkspaceMetadata{}, err
		}
		w.versions[rel] = 1
		if readOnly && isProjectReferenceReadOnlyPath(rel) {
			w.readOnly[rel] = true
		}
	}
	if err := w.writeReadOnlyManifestLocked(); err != nil {
		w.mu.Unlock()
		return WorkspaceMetadata{}, err
	}
	w.ExampleID = sourceID
	w.mu.Unlock()
	return w.Metadata()
}

func isProjectReferenceReadOnlyPath(path string) bool {
	return fileKind(path) != "anonymous"
}

func collectProjectReferenceFiles(ref ProjectReference) (map[string]string, error) {
	root := strings.TrimSpace(ref.Path)
	if root == "" {
		return nil, errors.New("project reference path is required")
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project reference is not a directory: %s", root)
	}
	files := make(map[string]string)
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if shouldSkipProjectReferenceDir(entry.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		rel := slashRel(root, path)
		if !isAllowedPlaygroundExtension(filepath.Ext(rel)) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxPlaygroundFileSize {
			return fmt.Errorf("project reference file exceeds %d byte limit: %s", maxPlaygroundFileSize, rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = string(data)
		return nil
	}); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("project reference has no loadable files: %s", root)
	}
	if _, ok := files["sfdx-project.json"]; !ok {
		files["sfdx-project.json"] = sfdxProjectJSON
	} else {
		files["sfdx-project.json"] = projectReferenceSFDXProjectJSON(files["sfdx-project.json"])
	}
	delete(files, "glade.yml")
	delete(files, "glade.yaml")
	if _, ok := files["anonymous.apex"]; !ok {
		files["anonymous.apex"] = "System.debug('Loaded local project reference');\n"
	}
	if _, ok := files["seed.json"]; !ok {
		files["seed.json"] = "{}\n"
	}
	return files, nil
}

func shouldSkipProjectReferenceDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case ".git", ".hg", ".svn", ".sf", ".sfdx", ".glade", "node_modules", "dist", "bin":
		return true
	default:
		return false
	}
}

func projectReferenceSFDXProjectJSON(content string) string {
	var cfg map[string]any
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		return sfdxProjectJSON
	}
	cfg["namespace"] = ""
	data, err := json.Marshal(cfg)
	if err != nil {
		return sfdxProjectJSON
	}
	return string(data) + "\n"
}

func (w *Workspace) refreshVersions() error {
	files, err := w.listFilesLocked()
	if err != nil {
		return err
	}
	for _, file := range files {
		if w.versions[file.Path] == 0 {
			w.versions[file.Path] = 1
		}
	}
	return nil
}

func (w *Workspace) loadReadOnlyManifest() error {
	path := filepath.Join(w.Root, readOnlyManifestFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var paths []string
	if err := json.Unmarshal(data, &paths); err != nil {
		return err
	}
	w.readOnly = make(map[string]bool, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
		if path != "" {
			w.readOnly[path] = true
		}
	}
	return nil
}

func (w *Workspace) writeReadOnlyManifestLocked() error {
	path := filepath.Join(w.Root, readOnlyManifestFile)
	if len(w.readOnly) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	paths := make([]string, 0, len(w.readOnly))
	for rel := range w.readOnly {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	return writeJSONFile(path, paths)
}

func (w *Workspace) SafePath(rel string) (string, error) {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "" {
		return "", errors.New("path is required")
	}
	if strings.HasPrefix(rel, "/") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("path escapes workspace: %s", rel)
	}
	ext := strings.ToLower(filepath.Ext(clean))
	if !isAllowedPlaygroundExtension(ext) {
		return "", fmt.Errorf("unsupported playground file extension %q", ext)
	}
	full := filepath.Join(w.Root, clean)
	root, err := filepath.Abs(w.Root)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace: %s", rel)
	}
	return abs, nil
}

func isAllowedPlaygroundExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".cls", ".trigger", ".apex", ".json", ".xml", ".yml", ".yaml":
		return true
	default:
		return false
	}
}

func (w *Workspace) SaveFile(req FileSaveRequest) (FileSaveResponse, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(req.Content) > maxPlaygroundFileSize {
		return FileSaveResponse{}, fmt.Errorf("file exceeds %d byte limit", maxPlaygroundFileSize)
	}
	path, err := w.SafePath(req.Path)
	if err != nil {
		return FileSaveResponse{}, err
	}
	rel := slashRel(w.Root, path)
	current := w.versions[rel]
	if w.readOnly[rel] {
		return FileSaveResponse{}, ErrReadOnlyFile{Path: rel}
	}
	if current > 0 && req.Version != current {
		return FileSaveResponse{}, ErrVersionConflict{Path: rel, Expected: current, Got: req.Version}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return FileSaveResponse{}, err
	}
	if err := os.WriteFile(path, []byte(req.Content), 0o644); err != nil {
		return FileSaveResponse{}, err
	}
	w.versions[rel] = current + 1
	if w.versions[rel] == 0 {
		w.versions[rel] = 1
	}
	files, err := w.listFilesLocked()
	if err != nil {
		return FileSaveResponse{}, err
	}
	hash, err := w.hashLocked(files)
	if err != nil {
		return FileSaveResponse{}, err
	}
	return FileSaveResponse{File: WorkspaceFile{Path: rel, Kind: fileKind(rel), Version: w.versions[rel], Size: int64(len(req.Content))}, WorkspaceHash: hash}, nil
}

func (w *Workspace) DeleteFile(rel string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	path, err := w.SafePath(rel)
	if err != nil {
		return err
	}
	rel = slashRel(w.Root, path)
	if w.readOnly[rel] {
		return ErrReadOnlyFile{Path: rel}
	}
	if filepath.Base(path) == "sfdx-project.json" {
		return errors.New("sfdx-project.json cannot be deleted")
	}
	delete(w.versions, rel)
	return os.Remove(path)
}

func (w *Workspace) Metadata() (WorkspaceMetadata, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	files, err := w.listFilesLocked()
	if err != nil {
		return WorkspaceMetadata{}, err
	}
	hash, err := w.hashLocked(files)
	if err != nil {
		return WorkspaceMetadata{}, err
	}
	anonymous, _ := os.ReadFile(filepath.Join(w.Root, "anonymous.apex"))
	exampleID := w.ExampleID
	if exampleID == "" {
		exampleID = w.detectExampleIDLocked(files)
	}
	return WorkspaceMetadata{
		ID:            w.ID,
		Root:          w.Root,
		ProjectRoot:   w.ProjectRoot,
		ExampleID:     exampleID,
		Files:         files,
		AnonymousBody: string(anonymous),
		WorkspaceHash: hash,
	}, nil
}

func (w *Workspace) detectExampleIDLocked(files []WorkspaceFile) string {
	if len(files) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		seen[file.Path] = struct{}{}
	}
	for _, example := range exampleProjects {
		if len(example.Files) != len(files) {
			continue
		}
		matched := true
		for rel, content := range example.Files {
			if _, ok := seen[rel]; !ok {
				matched = false
				break
			}
			data, err := os.ReadFile(filepath.Join(w.Root, filepath.FromSlash(rel)))
			if err != nil || string(data) != content {
				matched = false
				break
			}
		}
		if matched {
			return example.ID
		}
	}
	return ""
}

func (w *Workspace) Hash() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	files, err := w.listFilesLocked()
	if err != nil {
		return "", err
	}
	return w.hashLocked(files)
}

func (w *Workspace) RuntimeSourceHash() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	files, err := w.listFilesLocked()
	if err != nil {
		return "", err
	}
	return w.hashLocked(selectRuntimeSourceFiles(files))
}

func (w *Workspace) listFilesLocked() ([]WorkspaceFile, error) {
	var files []WorkspaceFile
	if err := filepath.WalkDir(w.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != w.Root && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		rel := slashRel(w.Root, path)
		if _, err := w.SafePath(rel); err != nil {
			if rel != "sfdx-project.json" {
				return nil
			}
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		version := w.versions[rel]
		if version == 0 {
			version = 1
		}
		files = append(files, WorkspaceFile{Path: rel, Kind: fileKind(rel), Version: version, Size: info.Size(), ReadOnly: w.readOnly[rel]})
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func (w *Workspace) hashLocked(files []WorkspaceFile) (string, error) {
	h := sha256.New()
	for _, file := range files {
		if file.Kind == "other" {
			continue
		}
		path := filepath.Join(w.Root, filepath.FromSlash(file.Path))
		body, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		h.Write([]byte(file.Path))
		h.Write([]byte{0})
		h.Write(body)
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func selectRuntimeSourceFiles(files []WorkspaceFile) []WorkspaceFile {
	out := make([]WorkspaceFile, 0, len(files))
	for _, file := range files {
		if file.Kind == "anonymous" || file.Kind == "data" {
			continue
		}
		out = append(out, file)
	}
	return out
}

func slashRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func fileKind(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cls":
		return "class"
	case ".trigger":
		return "trigger"
	case ".apex":
		return "anonymous"
	case ".json":
		if strings.EqualFold(filepath.Base(path), "seed.json") {
			return "data"
		}
		return "metadata"
	case ".xml", ".yml", ".yaml":
		return "metadata"
	default:
		return "other"
	}
}

type ErrVersionConflict struct {
	Path     string
	Expected int
	Got      int
}

func (e ErrVersionConflict) Error() string {
	return fmt.Sprintf("stale version for %s: got %d, want %d", e.Path, e.Got, e.Expected)
}

type ErrReadOnlyFile struct {
	Path string
}

func (e ErrReadOnlyFile) Error() string {
	return fmt.Sprintf("read-only project file: %s", e.Path)
}

func writeJSONFile(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
