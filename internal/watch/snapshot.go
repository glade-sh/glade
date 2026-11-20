package watch

import (
	"crypto/sha256"
	"errors"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/filestamp"
)

type FileMetadata struct {
	ModTimeUnixNano int64 `json:"modTimeUnixNano"`
	Size            int64 `json:"size"`
}

type FileState struct {
	contentHash [sha256.Size]byte
	stamp       filestamp.Stamp
	stampValid  bool
	Path        string       `json:"path"`
	Kind        FileKind     `json:"kind"`
	Name        string       `json:"name,omitempty"`
	ObjectName  string       `json:"objectName,omitempty"`
	Metadata    FileMetadata `json:"metadata"`
}

type Snapshot struct {
	Files    map[string]FileState     `json:"files"`
	Topology map[string]TopologyState `json:"topology,omitempty"`
}

type TopologyState struct {
	Path   string      `json:"path"`
	Mode   os.FileMode `json:"mode"`
	Target string      `json:"target,omitempty"`
}

type ChangeOp string

const (
	ChangeAdded    ChangeOp = "added"
	ChangeModified ChangeOp = "modified"
	ChangeDeleted  ChangeOp = "deleted"
)

type Change struct {
	Path       string        `json:"path"`
	Op         ChangeOp      `json:"op"`
	Kind       FileKind      `json:"kind"`
	Name       string        `json:"name,omitempty"`
	ObjectName string        `json:"objectName,omitempty"`
	Before     *FileMetadata `json:"before,omitempty"`
	After      *FileMetadata `json:"after,omitempty"`
}

func CaptureSnapshot(root string) (Snapshot, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Snapshot{}, err
	}
	files := make(map[string]FileState)
	var capturer fileCapturer
	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		state, ok, err := capturer.capturePath(path)
		if err != nil || !ok {
			return err
		}
		files[state.Path] = state
		return nil
	})
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Files: files}, nil
}

// CaptureScope captures watchable product files below the scoped roots and the
// exact configuration files named by the scope. Missing roots and files are
// valid so their later creation can be observed.
func CaptureScope(scope Scope) (Snapshot, error) {
	return captureScopeWithPrevious(scope, Snapshot{})
}

// Only polling reuses old, verified local-file stamps. Native events always
// read the affected files, including full reconciliations.
func captureScopeWithPrevious(scope Scope, previous Snapshot) (Snapshot, error) {
	scope = NormalizeScope(scope)
	files := make(map[string]FileState)
	capturer := fileCapturer{previous: previous.Files, stampFiles: true, now: time.Now()}
	for _, root := range scope.Roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if scopeExcludesPath(scope, path) {
				if d.IsDir() {
					if scopeNeedsExcludedTraversal(scope, path) {
						return nil
					}
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			classification := ClassifyPath(path)
			if classification.Kind == FileKindIgnored && !isScopeConfigPath(path, root) {
				return nil
			}
			state, ok, err := capturer.capturePath(path)
			if err != nil || !ok {
				return err
			}
			files[state.Path] = state
			return nil
		})
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, err
		}
	}
	for _, path := range scope.Files {
		state, ok, err := capturer.capturePath(path)
		if err != nil {
			return Snapshot{}, err
		}
		if ok {
			files[state.Path] = state
		}
	}
	topology, err := captureScopeTopology(scope)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Files: files, Topology: topology}, nil
}

func scopeExcludesPath(scope Scope, path string) bool {
	for _, excluded := range scope.ExcludedRoots {
		if !pathWithin(path, excluded) {
			continue
		}
		carvedOut := false
		for _, exception := range scope.ExclusionExceptions {
			if pathWithin(path, exception) && pathWithin(exception, excluded) {
				carvedOut = true
				break
			}
		}
		if !carvedOut {
			return true
		}
	}
	return false
}

func scopeNeedsExcludedTraversal(scope Scope, path string) bool {
	for _, exception := range scope.ExclusionExceptions {
		if pathWithin(exception, path) {
			return true
		}
	}
	return false
}

func captureScopeTopology(scope Scope) (map[string]TopologyState, error) {
	var topology map[string]TopologyState
	for _, path := range scope.Topology {
		state, ok, err := captureTopology(path)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if topology == nil {
			topology = make(map[string]TopologyState)
		}
		topology[state.Path] = state
	}
	return topology, nil
}

func isRootConfigPath(path, root string) bool {
	return path == filepath.Join(root, "sfdx-project.json") || path == filepath.Join(root, "glade.yml")
}

// isScopeConfigPath adds .forceignore files at any depth below root: the
// nearest one decides which package source loads.
func isScopeConfigPath(path, root string) bool {
	return isRootConfigPath(path, root) || (strings.EqualFold(filepath.Base(path), ".forceignore") && pathWithin(path, root))
}

func CapturePaths(paths []string) (Snapshot, error) {
	files := make(map[string]FileState)
	var capturer fileCapturer
	for _, path := range paths {
		state, ok, err := capturer.capturePath(path)
		if err != nil {
			return Snapshot{}, err
		}
		if ok {
			files[state.Path] = state
		}
	}
	return Snapshot{Files: files}, nil
}

func DiffSnapshots(before, after Snapshot) []Change {
	var changes []Change

	for path, beforeState := range before.Files {
		afterState, ok := after.Files[path]
		if !ok {
			changes = append(changes, changeFromStates(ChangeDeleted, &beforeState, nil))
			continue
		}
		if beforeState.Metadata != afterState.Metadata || beforeState.contentHash != afterState.contentHash {
			changes = append(changes, changeFromStates(ChangeModified, &beforeState, &afterState))
		}
	}
	for path, afterState := range after.Files {
		if _, ok := before.Files[path]; ok {
			continue
		}
		changes = append(changes, changeFromStates(ChangeAdded, nil, &afterState))
	}
	for path, beforeState := range before.Topology {
		afterState, ok := after.Topology[path]
		if !ok {
			changes = append(changes, topologyChange(ChangeDeleted, beforeState.Path))
			continue
		}
		if beforeState != afterState {
			changes = append(changes, topologyChange(ChangeModified, afterState.Path))
		}
	}
	for path, afterState := range after.Topology {
		if _, ok := before.Topology[path]; !ok {
			changes = append(changes, topologyChange(ChangeAdded, afterState.Path))
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path == changes[j].Path {
			return changes[i].Op < changes[j].Op
		}
		return changes[i].Path < changes[j].Path
	})
	return changes
}

func topologyChange(op ChangeOp, path string) Change {
	return Change{Path: path, Op: op, Kind: FileKindTopology, Name: filepath.Base(path)}
}

func captureTopology(path string) (TopologyState, bool, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return TopologyState{}, false, err
	}
	absPath = filepath.Clean(absPath)
	info, err := os.Lstat(absPath)
	if errors.Is(err, os.ErrNotExist) {
		return TopologyState{}, false, nil
	}
	if err != nil {
		return TopologyState{}, false, err
	}
	state := TopologyState{Path: absPath, Mode: info.Mode()}
	if info.Mode()&os.ModeSymlink != 0 {
		state.Target, err = os.Readlink(absPath)
		if err != nil {
			return TopologyState{}, false, err
		}
	}
	return state, true, nil
}

type fileCapturer struct {
	buffer     []byte
	digest     hash.Hash
	previous   map[string]FileState
	stampFiles bool
	now        time.Time
}

func (c *fileCapturer) capturePath(path string) (FileState, bool, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return FileState{}, false, err
	}
	classification := ClassifyPath(absPath)
	if !classification.Watchable {
		return FileState{}, false, nil
	}

	var stamp filestamp.Stamp
	var stamped bool
	var info os.FileInfo
	if c.stampFiles {
		info, stamp, stamped = filestamp.StatWithInfo(absPath)
	}
	if info == nil {
		info, err = os.Stat(absPath)
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileState{}, false, nil
		}
		return FileState{}, false, err
	}
	if info.IsDir() {
		return FileState{}, false, nil
	}

	var contentHash [sha256.Size]byte
	var stableStamp bool
	if info.Mode().IsRegular() {
		if old, ok := c.previous[absPath]; ok && stamped && old.stampValid && old.stamp == stamp {
			contentHash, stableStamp = old.contentHash, true
		} else {
			file, err := os.Open(absPath)
			if errors.Is(err, os.ErrNotExist) {
				return FileState{}, false, nil
			}
			if err != nil {
				return FileState{}, false, err
			}
			var before filestamp.Stamp
			var beforeOK bool
			if stamped && stamp.OlderThan(c.now, 2*time.Second) {
				before, beforeOK = filestamp.Fstat(file)
			}
			if c.buffer == nil {
				c.buffer = make([]byte, 32*1024)
				c.digest = sha256.New()
			}
			c.digest.Reset()
			for {
				n, readErr := file.Read(c.buffer)
				if n > 0 {
					_, _ = c.digest.Write(c.buffer[:n])
				}
				if readErr != nil {
					if readErr != io.EOF {
						_ = file.Close()
						return FileState{}, false, readErr
					}
					break
				}
			}
			c.digest.Sum(contentHash[:0])
			if beforeOK && before == stamp {
				after, ok := filestamp.Fstat(file)
				stableStamp = ok && after == stamp
			}
			_ = file.Close()
		}
	}

	return FileState{
		contentHash: contentHash,
		stamp:       stamp,
		stampValid:  stableStamp,
		Path:        absPath,
		Kind:        classification.Kind,
		Name:        classification.Name,
		ObjectName:  classification.ObjectName,
		Metadata: FileMetadata{
			ModTimeUnixNano: info.ModTime().UnixNano(),
			Size:            info.Size(),
		},
	}, true, nil
}

func changeFromStates(op ChangeOp, before, after *FileState) Change {
	state := before
	if after != nil {
		state = after
	}

	change := Change{
		Path:       state.Path,
		Op:         op,
		Kind:       state.Kind,
		Name:       state.Name,
		ObjectName: state.ObjectName,
	}
	if before != nil {
		metadata := before.Metadata
		change.Before = &metadata
	}
	if after != nil {
		metadata := after.Metadata
		change.After = &metadata
	}
	return change
}
