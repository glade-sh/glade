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
	SchemaVersion int       `json:"schemaVersion"`
	ProjectRoot   string    `json:"projectRoot"`
	SourceHash    string    `json:"sourceHash"`
	CreatedAt     time.Time `json:"createdAt"`
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
	sourceHash, err := sourceHashForGraph(root, graph)
	if err != nil {
		return err
	}
	entry := cacheEntry{
		Metadata: CacheMetadata{
			SchemaVersion: cacheSchemaVersion,
			ProjectRoot:   root,
			SourceHash:    sourceHash,
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
	graph := BuildDeclarations(typesys.Index{
		Project:               typesys.ProjectInfo{Root: root},
		Objects:               s.Objects,
		CustomMetadataRecords: s.CustomMetadataRecords,
	})
	return WriteCache(root, graph)
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
	_, meta, err := ReadCache(projectRoot)
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
	sourceHash, err := sourceHashForIndex(root, index)
	if err != nil {
		return false
	}
	return meta.SourceHash == sourceHash
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

func sourceHashForGraph(projectRoot string, graph Graph) (string, error) {
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
	return sourceHashForPaths(projectRoot, sortedCachePaths(paths))
}

func sourceHashForIndex(projectRoot string, index typesys.Index) (string, error) {
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
	return sourceHashForPaths(projectRoot, sortedCachePaths(paths))
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

func sourceHashForPaths(projectRoot string, paths []string) (string, error) {
	hash := sha256.New()
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
		info, err := os.Stat(abs)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				io.WriteString(hash, rel)
				io.WriteString(hash, "\x00missing\x00")
				continue
			}
			return "", err
		}
		if info.IsDir() {
			continue
		}
		io.WriteString(hash, filepath.ToSlash(rel))
		io.WriteString(hash, "\x00")
		io.WriteString(hash, info.ModTime().UTC().Format(time.RFC3339Nano))
		io.WriteString(hash, "\x00")
		io.WriteString(hash, info.Mode().String())
		io.WriteString(hash, "\x00")
		io.WriteString(hash, formatCacheSize(info.Size()))
		io.WriteString(hash, "\x00")
		data, err := os.ReadFile(abs)
		if err != nil {
			return "", err
		}
		hash.Write(data)
		io.WriteString(hash, "\x00")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func formatCacheSize(size int64) string {
	return strconv.FormatInt(size, 10)
}
