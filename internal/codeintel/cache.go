package codeintel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

const cacheSchemaVersion = 1

const cacheIndexFile = "index.json"

var ErrCacheMiss = errors.New("codeintel cache miss")

type CacheMetadata struct {
	SchemaVersion int               `json:"schemaVersion"`
	ProjectRoot   string            `json:"projectRoot"`
	SourceHash    string            `json:"sourceHash"`
	SourceFiles   []CacheSourceFile `json:"sourceFiles"`
	CreatedAt     time.Time         `json:"createdAt"`
}

type CacheSourceFile struct {
	Path            string `json:"path"`
	ModTimeUnixNano int64  `json:"modTimeUnixNano,omitempty"`
	Size            int64  `json:"size,omitempty"`
	Mode            string `json:"mode,omitempty"`
	Missing         bool   `json:"missing,omitempty"`
	ContentHash     string `json:"contentHash,omitempty"`
}

type cacheEntry struct {
	Metadata CacheMetadata `json:"metadata"`
	Graph    Graph         `json:"graph"`
}

func CacheDir(projectRoot string) string {
	root, err := filepath.Abs(projectRoot)
	if err == nil {
		projectRoot = root
	}
	return filepath.Join(filepath.Clean(projectRoot), ".glade", "symbols")
}

func WriteCache(projectRoot string, graph Graph) error {
	root, err := cleanAbsRoot(projectRoot)
	if err != nil {
		return err
	}
	if graph.ProjectRoot == "" {
		graph.ProjectRoot = root
	}
	sourceHash, sourceFiles, err := sourceFingerprintForGraph(root, graph)
	if err != nil {
		return err
	}
	return writeCacheEntry(root, graph, sourceHash, sourceFiles)
}

func writeCacheEntry(root string, graph Graph, sourceHash string, sourceFiles []CacheSourceFile) error {
	entry := cacheEntry{
		Metadata: CacheMetadata{
			SchemaVersion: cacheSchemaVersion,
			ProjectRoot:   root,
			SourceHash:    sourceHash,
			SourceFiles:   sourceFiles,
			CreatedAt:     time.Now().UTC(),
		},
		Graph: graph,
	}
	dir := CacheDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, cacheIndexFile)
	tmp, err := os.CreateTemp(dir, "index-*.json")
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

func WriteSchemaCache(projectRoot string, s schema.Schema) error {
	root, err := cleanAbsRoot(projectRoot)
	if err != nil {
		return err
	}
	index := typesys.Index{
		Project:               typesys.ProjectInfo{Root: root},
		Objects:               s.Objects,
		CustomMetadataRecords: s.CustomMetadataRecords,
	}
	graph := BuildDeclarations(index)
	sourceHash, sourceFiles, err := sourceFingerprintForIndex(root, index)
	if err != nil {
		return err
	}
	return writeCacheEntry(root, graph, sourceHash, sourceFiles)
}

func ReadCache(projectRoot string) (Graph, CacheMetadata, error) {
	root, err := cleanAbsRoot(projectRoot)
	if err != nil {
		return Graph{}, CacheMetadata{}, err
	}
	data, err := os.ReadFile(filepath.Join(CacheDir(root), cacheIndexFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Graph{}, CacheMetadata{}, ErrCacheMiss
		}
		return Graph{}, CacheMetadata{}, err
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return Graph{}, CacheMetadata{}, err
	}
	if entry.Metadata.SchemaVersion != cacheSchemaVersion {
		return Graph{}, CacheMetadata{}, ErrCacheMiss
	}
	if filepath.Clean(entry.Metadata.ProjectRoot) != root {
		return Graph{}, CacheMetadata{}, ErrCacheMiss
	}
	return entry.Graph, entry.Metadata, nil
}

func CacheFresh(projectRoot string, index typesys.Index) bool {
	graph, meta, err := ReadCache(projectRoot)
	if err != nil {
		return false
	}
	root, err := cleanAbsRoot(projectRoot)
	if err != nil {
		return false
	}
	if meta.SchemaVersion != cacheSchemaVersion || filepath.Clean(meta.ProjectRoot) != root {
		return false
	}
	if meta.SourceFiles != nil {
		sourceFiles, fresh, err := freshSourceFiles(root, meta.SourceFiles, cachePathsForIndex(index))
		if err != nil || !fresh {
			return false
		}
		sourceHash, err := sourceHashForFingerprints(root, sourceFiles, cacheShapeForIndex(index))
		if err != nil {
			return false
		}
		if meta.SourceHash == sourceHash {
			return true
		}
	} else {
		sourceHash, _, err := sourceFingerprintForIndex(root, index)
		if err != nil {
			return false
		}
		if meta.SourceHash == sourceHash {
			return true
		}
	}
	if len(index.Diagnostics) > 0 {
		return false
	}
	graphHash, _, err := sourceFingerprintForGraph(root, graph)
	if err != nil || meta.SourceHash != graphHash {
		return false
	}
	current := BuildDeclarations(index)
	addArtifactContracts(&current, index.CodeIntelSymbols, index.CodeIntelUses)
	return declarationShapeHash(graph) == declarationShapeHash(current)
}

func ClearCache(projectRoot string) error {
	return os.RemoveAll(CacheDir(projectRoot))
}

func cleanAbsRoot(projectRoot string) (string, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	return filepath.Clean(root), nil
}

func sourceFingerprintForGraph(projectRoot string, graph Graph) (string, []CacheSourceFile, error) {
	paths := make(map[string]struct{})
	for _, symbol := range graph.Symbols {
		addCachePath(paths, symbol.File)
	}
	for _, use := range graph.Uses {
		addCachePath(paths, use.File)
	}
	for _, diag := range graph.Diagnostics {
		addCachePath(paths, diag.File)
	}
	sourceFiles, err := sourceFingerprintsForPaths(projectRoot, sortedCachePaths(paths), true)
	if err != nil {
		return "", nil, err
	}
	var shape []byte
	if !graphHasContractInputs(graph) {
		hash, err := sourceHashForFingerprints(projectRoot, sourceFiles, nil)
		return hash, sourceFiles, err
	}
	shape, err = json.Marshal(struct {
		Symbols map[SymbolID]Symbol `json:"symbols,omitempty"`
		Uses    []Use               `json:"uses,omitempty"`
	}{
		Symbols: graph.Symbols,
		Uses:    graph.Uses,
	})
	if err != nil {
		return "", nil, err
	}
	hash, err := sourceHashForFingerprints(projectRoot, sourceFiles, shape)
	return hash, sourceFiles, err
}

func graphHasContractInputs(graph Graph) bool {
	for _, symbol := range graph.Symbols {
		switch symbol.Kind {
		case SymbolSObject, SymbolSObjectField, SymbolCustomMetadata, SymbolLabel, SymbolStaticResource:
			return true
		}
		if symbol.Dependency || symbol.Artifact {
			return true
		}
	}
	for _, use := range graph.Uses {
		switch use.Kind {
		case UseQuery, UseMutate, UseMetadata:
			return true
		}
	}
	return false
}

func declarationShapeHash(graph Graph) string {
	shape := struct {
		Symbols []Symbol `json:"symbols,omitempty"`
		Uses    []Use    `json:"uses,omitempty"`
	}{
		Symbols: graph.SortedSymbols(),
	}
	for _, use := range graph.Uses {
		if use.Kind == UseDeclaration {
			shape.Uses = append(shape.Uses, use)
		}
	}
	sortUses(shape.Uses)
	data, err := json.Marshal(shape)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sourceFingerprintForIndex(projectRoot string, index typesys.Index) (string, []CacheSourceFile, error) {
	sourceFiles, err := sourceFingerprintsForPaths(projectRoot, cachePathsForIndex(index), true)
	if err != nil {
		return "", nil, err
	}
	hash, err := sourceHashForFingerprints(projectRoot, sourceFiles, cacheShapeForIndex(index))
	return hash, sourceFiles, err
}

func cachePathsForIndex(index typesys.Index) []string {
	paths := make(map[string]struct{})
	for _, typ := range index.Types {
		addCachePath(paths, typ.File)
	}
	for _, trigger := range index.Triggers {
		addCachePath(paths, trigger.File)
	}
	for _, diag := range index.Diagnostics {
		addCachePath(paths, diag.File)
	}
	for _, dep := range index.Dependencies {
		addCachePath(paths, dep.SourceRoot)
	}
	return sortedCachePaths(paths)
}

func cacheShapeForIndex(index typesys.Index) []byte {
	if len(index.Objects) == 0 && len(index.CustomMetadataRecords) == 0 && len(index.CodeIntelSymbols) == 0 && len(index.CodeIntelUses) == 0 && len(index.Dependencies) == 0 {
		return nil
	}
	shape := struct {
		Objects               any `json:"objects,omitempty"`
		CustomMetadataRecords any `json:"customMetadataRecords,omitempty"`
		CodeIntelSymbols      any `json:"codeIntelSymbols,omitempty"`
		CodeIntelUses         any `json:"codeIntelUses,omitempty"`
		Dependencies          any `json:"dependencies,omitempty"`
	}{
		Objects:               index.Objects,
		CustomMetadataRecords: index.CustomMetadataRecords,
		CodeIntelSymbols:      index.CodeIntelSymbols,
		CodeIntelUses:         index.CodeIntelUses,
		Dependencies:          index.Dependencies,
	}
	data, err := json.Marshal(shape)
	if err != nil {
		return nil
	}
	return data
}

func addCachePath(paths map[string]struct{}, path string) {
	if path == "" {
		return
	}
	paths[filepath.Clean(path)] = struct{}{}
}

func sortedCachePaths(paths map[string]struct{}) []string {
	out := make([]string, 0, len(paths))
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func freshSourceFiles(projectRoot string, cached []CacheSourceFile, paths []string) ([]CacheSourceFile, bool, error) {
	current, err := sourceFingerprintsForPaths(projectRoot, paths, false)
	if err != nil {
		return nil, false, err
	}
	if len(current) != len(cached) {
		return nil, false, nil
	}
	cachedByPath := make(map[string]CacheSourceFile, len(cached))
	for _, file := range cached {
		cachedByPath[file.Path] = file
	}
	for i, file := range current {
		cachedFile, ok := cachedByPath[file.Path]
		if !ok {
			return nil, false, nil
		}
		if file.Missing || cachedFile.Missing {
			if file.Missing == cachedFile.Missing {
				current[i].ContentHash = cachedFile.ContentHash
				continue
			}
			return nil, false, nil
		}
		if sameSourceFileStat(file, cachedFile) {
			current[i].ContentHash = cachedFile.ContentHash
			continue
		}
		if file.Size != cachedFile.Size {
			return nil, false, nil
		}
		contentHash, err := contentHashForCachePath(projectRoot, file.Path)
		if err != nil {
			return nil, false, err
		}
		if contentHash != cachedFile.ContentHash {
			return nil, false, nil
		}
		current[i].ContentHash = contentHash
	}
	return current, true, nil
}

func sameSourceFileStat(a, b CacheSourceFile) bool {
	return a.ModTimeUnixNano == b.ModTimeUnixNano && a.Size == b.Size && a.Mode == b.Mode
}

func sourceFingerprintsForPaths(projectRoot string, paths []string, includeContent bool) ([]CacheSourceFile, error) {
	files := make([]CacheSourceFile, 0, len(paths))
	for _, path := range paths {
		abs := path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(projectRoot, path)
		}
		abs = filepath.Clean(abs)
		rel := path
		if filepath.IsAbs(path) {
			if r, err := filepath.Rel(projectRoot, abs); err == nil {
				rel = r
			}
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		info, err := os.Stat(abs)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				files = append(files, CacheSourceFile{Path: rel, Missing: true})
				continue
			}
			return nil, err
		}
		if info.IsDir() {
			continue
		}
		file := CacheSourceFile{
			Path:            rel,
			ModTimeUnixNano: info.ModTime().UnixNano(),
			Size:            info.Size(),
			Mode:            info.Mode().String(),
		}
		if includeContent {
			contentHash, err := contentHashForAbsPath(abs)
			if err != nil {
				return nil, err
			}
			file.ContentHash = contentHash
		}
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func sourceHashForFingerprints(projectRoot string, files []CacheSourceFile, shape []byte) (string, error) {
	hash := sha256.New()
	for _, file := range files {
		io.WriteString(hash, file.Path)
		io.WriteString(hash, "\x00")
		if file.Missing {
			io.WriteString(hash, "missing")
			io.WriteString(hash, "\x00")
			continue
		}
		contentHash := file.ContentHash
		if contentHash == "" {
			var err error
			contentHash, err = contentHashForCachePath(projectRoot, file.Path)
			if err != nil {
				return "", err
			}
		}
		io.WriteString(hash, formatCacheSize(file.Size))
		io.WriteString(hash, "\x00")
		io.WriteString(hash, contentHash)
		io.WriteString(hash, "\x00")
	}
	if len(shape) > 0 {
		hash.Write(shape)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func contentHashForCachePath(projectRoot, path string) (string, error) {
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(projectRoot, filepath.FromSlash(path))
	}
	return contentHashForAbsPath(abs)
}

func contentHashForAbsPath(abs string) (string, error) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func formatCacheSize(size int64) string {
	return strconv.FormatInt(size, 10)
}
