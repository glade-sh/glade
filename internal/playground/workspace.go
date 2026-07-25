package playground

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	if managed {
		if err := validateWorkspaceID(id); err != nil {
			return nil, err
		}
	}
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

func validateWorkspaceID(id string) error {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\\`) || filepath.IsAbs(id) {
		return fmt.Errorf("invalid workspace ID %q", id)
	}
	return nil
}

func (w *Workspace) openRoot() (*os.Root, error) {
	if w.Managed {
		return w.openManagedRoot(false)
	}
	if err := os.MkdirAll(w.Root, 0o750); err != nil {
		return nil, err
	}
	return os.OpenRoot(w.Root)
}

func (w *Workspace) openManagedRoot(reset bool) (*os.Root, error) {
	if err := validateWorkspaceID(w.ID); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(w.DataRoot, 0o700); err != nil {
		return nil, err
	}
	data, err := os.OpenRoot(w.DataRoot)
	if err != nil {
		return nil, err
	}
	defer data.Close()
	workspaces, err := openVerifiedDirectory(data, "workspaces")
	if err != nil {
		return nil, err
	}
	defer workspaces.Close()
	workspace, err := openVerifiedDirectory(workspaces, w.ID)
	if err != nil {
		return nil, err
	}
	if reset {
		if err := clearRoot(workspace); err != nil {
			workspace.Close()
			return nil, err
		}
	}
	return workspace, nil
}

func openVerifiedDirectory(parent *os.Root, name string) (*os.Root, error) {
	info, err := parent.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		if err := parent.Mkdir(name, 0o755); err != nil {
			return nil, err
		}
		info, err = parent.Lstat(name)
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("directory is not a safe directory: %s", name)
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	opened, err := child.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		child.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("directory changed while opening: %s", name)
	}
	return child, nil
}

func clearRoot(root *os.Root) error {
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := root.RemoveAll(entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func normalizedWorkspacePath(rel string) (string, error) {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "" || strings.HasPrefix(rel, "/") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path is required or absolute: %s", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace: %s", rel)
	}
	if !isAllowedPlaygroundExtension(filepath.Ext(clean)) {
		return "", fmt.Errorf("unsupported playground file extension %q", filepath.Ext(clean))
	}
	return filepath.ToSlash(clean), nil
}

func confinedLstat(root *os.Root, rel string) (os.FileInfo, error) {
	if err := rejectSymlinkComponents(root, rel); err != nil {
		return nil, err
	}
	info, err := root.Lstat(rel)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("symbolic links are not allowed: %s", rel)
	}
	return info, nil
}

func rejectSymlinkComponents(root *os.Root, rel string) error {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i := range parts {
		name := strings.Join(parts[:i+1], "/")
		info, err := root.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not allowed: %s", name)
		}
	}
	return nil
}

func (w *Workspace) openFile(rel string) (*os.File, os.FileInfo, error) {
	rel, err := normalizedWorkspacePath(rel)
	if err != nil {
		return nil, nil, err
	}
	root, err := w.openRoot()
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	if _, err := confinedLstat(root, rel); err != nil {
		return nil, nil, err
	}
	file, err := root.Open(rel)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, nil, fmt.Errorf("not a regular file: %s", rel)
	}
	return file, info, nil
}

func (w *Workspace) ensureDefaultFiles() error {
	root, err := w.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	hasFiles := false
	if err := fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if path != "." && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
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
		if err := rejectSymlinkComponents(root, rel); err != nil {
			return err
		}
		if _, err := root.Lstat(rel); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := root.MkdirAll(filepath.ToSlash(filepath.Dir(rel)), 0o755); err != nil {
			return err
		}
		if err := root.WriteFile(rel, []byte(content), 0o644); err != nil {
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
	return w.LoadProjectReferenceLimited(ref, 0, 0)
}

func (w *Workspace) LoadProjectReferenceLimited(ref ProjectReference, maxFiles int, maxBytes int64) (WorkspaceMetadata, error) {
	files, err := collectProjectReferenceFilesLimited(ref, maxFiles, maxBytes)
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
	if err := validateWorkspaceID(w.ID); err != nil {
		w.mu.Unlock()
		return WorkspaceMetadata{}, err
	}
	root, err := w.openManagedRoot(true)
	if err != nil {
		w.mu.Unlock()
		return WorkspaceMetadata{}, err
	}
	defer root.Close()
	w.versions = make(map[string]int)
	w.readOnly = make(map[string]bool)
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		rel, err := normalizedWorkspacePath(rel)
		if err != nil {
			w.mu.Unlock()
			return WorkspaceMetadata{}, err
		}
		if err := root.MkdirAll(filepath.ToSlash(filepath.Dir(rel)), 0o755); err != nil {
			w.mu.Unlock()
			return WorkspaceMetadata{}, err
		}
		if err := root.WriteFile(rel, []byte(files[rel]), 0o644); err != nil {
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
	return collectProjectReferenceFilesLimited(ref, 0, 0)
}

func collectProjectReferenceFilesLimited(ref ProjectReference, maxFiles int, maxBytes int64) (map[string]string, error) {
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
	confined, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer confined.Close()
	files := make(map[string]string)
	var totalBytes int64
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
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		rel := slashRel(root, path)
		if !isAllowedPlaygroundExtension(filepath.Ext(rel)) {
			return nil
		}
		info, err := confinedLstat(confined, rel)
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil
		}
		if info.Size() > maxPlaygroundFileSize {
			return fmt.Errorf("project reference file exceeds %d byte limit: %s", maxPlaygroundFileSize, rel)
		}
		if maxFiles > 0 && len(files)+1 > maxFiles {
			return fmt.Errorf("project reference file limit exceeded: %d", maxFiles)
		}
		if maxBytes > 0 && totalBytes+info.Size() > maxBytes {
			return fmt.Errorf("project reference size limit exceeded: %d bytes", maxBytes)
		}
		file, err := confined.Open(rel)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maxPlaygroundFileSize+1))
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if len(data) > maxPlaygroundFileSize {
			return fmt.Errorf("project reference file exceeds %d byte limit: %s", maxPlaygroundFileSize, rel)
		}
		if maxBytes > 0 && totalBytes+int64(len(data)) > maxBytes {
			return fmt.Errorf("project reference size limit exceeded: %d bytes", maxBytes)
		}
		files[rel] = string(data)
		totalBytes += int64(len(data))
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
	if err := enforceProjectReferenceLimits(files, maxFiles, maxBytes); err != nil {
		return nil, err
	}
	return files, nil
}

func enforceProjectReferenceLimits(files map[string]string, maxFiles int, maxBytes int64) error {
	if maxFiles > 0 && len(files) > maxFiles {
		return fmt.Errorf("project reference file limit exceeded: %d", maxFiles)
	}
	if maxBytes > 0 {
		var total int64
		for _, content := range files {
			total += int64(len(content))
		}
		if total > maxBytes {
			return fmt.Errorf("project reference size limit exceeded: %d bytes", maxBytes)
		}
	}
	return nil
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
	root, err := w.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	if _, err := confinedLstat(root, readOnlyManifestFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := root.ReadFile(readOnlyManifestFile)
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
	root, err := w.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	if len(w.readOnly) == 0 {
		if _, err := confinedLstat(root, readOnlyManifestFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := root.Remove(readOnlyManifestFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	paths := make([]string, 0, len(w.readOnly))
	for rel := range w.readOnly {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	data, err := json.MarshalIndent(paths, "", "  ")
	if err != nil {
		return err
	}
	if _, err := confinedLstat(root, readOnlyManifestFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return root.WriteFile(readOnlyManifestFile, append(data, '\n'), 0o644)
}

func (w *Workspace) SafePath(rel string) (string, error) {
	clean, err := normalizedWorkspacePath(rel)
	if err != nil {
		return "", err
	}
	full := filepath.Join(w.Root, filepath.FromSlash(clean))
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
	rel, err := normalizedWorkspacePath(req.Path)
	if err != nil {
		return FileSaveResponse{}, err
	}
	current := w.versions[rel]
	if w.readOnly[rel] {
		return FileSaveResponse{}, ErrReadOnlyFile{Path: rel}
	}
	if current > 0 && req.Version != current {
		return FileSaveResponse{}, ErrVersionConflict{Path: rel, Expected: current, Got: req.Version}
	}
	root, err := w.openRoot()
	if err != nil {
		return FileSaveResponse{}, err
	}
	defer root.Close()
	if info, err := root.Lstat(rel); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return FileSaveResponse{}, fmt.Errorf("symbolic links are not allowed: %s", rel)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return FileSaveResponse{}, err
	}
	if err := rejectSymlinkComponents(root, rel); err != nil {
		return FileSaveResponse{}, err
	}
	if err := root.MkdirAll(filepath.ToSlash(filepath.Dir(rel)), 0o755); err != nil {
		return FileSaveResponse{}, err
	}
	if err := root.WriteFile(rel, []byte(req.Content), 0o644); err != nil {
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

	rel, err := normalizedWorkspacePath(rel)
	if err != nil {
		return err
	}
	if w.readOnly[rel] {
		return ErrReadOnlyFile{Path: rel}
	}
	if filepath.Base(rel) == "sfdx-project.json" {
		return errors.New("sfdx-project.json cannot be deleted")
	}
	root, err := w.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	if _, err := confinedLstat(root, rel); err != nil {
		return err
	}
	delete(w.versions, rel)
	return root.Remove(rel)
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
	anonymous := []byte(nil)
	if root, err := w.openRoot(); err == nil {
		if _, err := confinedLstat(root, "anonymous.apex"); err == nil {
			if data, err := root.ReadFile("anonymous.apex"); err == nil {
				anonymous = data
			}
		}
		root.Close()
	}
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
			file, _, err := w.openFile(rel)
			if err != nil {
				matched = false
				break
			}
			data, readErr := io.ReadAll(file)
			file.Close()
			if readErr != nil || string(data) != content {
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
	root, err := w.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if err := fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != "." && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		rel, err := normalizedWorkspacePath(path)
		if err != nil {
			return nil
		}
		info, err := confinedLstat(root, rel)
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
	root, err := w.openRoot()
	if err != nil {
		return "", err
	}
	defer root.Close()
	h := sha256.New()
	for _, file := range files {
		if file.Kind == "other" {
			continue
		}
		if _, err := confinedLstat(root, file.Path); err != nil {
			return "", err
		}
		body, err := root.ReadFile(file.Path)
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

func (w *Workspace) FileHash(rel string) string {
	file, _, err := w.openFile(rel)
	if err != nil {
		return ""
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
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
