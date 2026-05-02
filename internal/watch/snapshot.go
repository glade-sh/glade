package watch

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
)

type FileMetadata struct {
	ModTimeUnixNano int64 `json:"modTimeUnixNano"`
	Size            int64 `json:"size"`
}

type FileState struct {
	Path       string       `json:"path"`
	Kind       FileKind     `json:"kind"`
	Name       string       `json:"name,omitempty"`
	ObjectName string       `json:"objectName,omitempty"`
	Metadata   FileMetadata `json:"metadata"`
}

type Snapshot struct {
	Files map[string]FileState `json:"files"`
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
	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		state, ok, err := capturePath(path)
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

func CapturePaths(paths []string) (Snapshot, error) {
	files := make(map[string]FileState)
	for _, path := range paths {
		state, ok, err := capturePath(path)
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
		if beforeState.Metadata != afterState.Metadata {
			changes = append(changes, changeFromStates(ChangeModified, &beforeState, &afterState))
		}
	}
	for path, afterState := range after.Files {
		if _, ok := before.Files[path]; ok {
			continue
		}
		changes = append(changes, changeFromStates(ChangeAdded, nil, &afterState))
	}

	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path == changes[j].Path {
			return changes[i].Op < changes[j].Op
		}
		return changes[i].Path < changes[j].Path
	})
	return changes
}

func capturePath(path string) (FileState, bool, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return FileState{}, false, err
	}
	classification := ClassifyPath(absPath)
	if !classification.Watchable {
		return FileState{}, false, nil
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileState{}, false, nil
		}
		return FileState{}, false, err
	}
	if info.IsDir() {
		return FileState{}, false, nil
	}

	return FileState{
		Path:       absPath,
		Kind:       classification.Kind,
		Name:       classification.Name,
		ObjectName: classification.ObjectName,
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
