package startupcache

import (
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
)

const stateGobFile = "startup.gob"

func Clear(projectRoot, subdir string) error {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return err
	}
	root = filepath.Clean(root)
	path := filepath.Join(root, filepath.FromSlash(subdir), stateGobFile)
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func readGob(projectRoot, subdir string) (*Entry, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	path := filepath.Join(root, filepath.FromSlash(subdir), stateGobFile)
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	var entry Entry
	dec := gob.NewDecoder(file)
	if err := dec.Decode(&entry); err != nil {
		return nil, nil
	}
	return &entry, nil
}

func writeGob(entry *Entry, subdir string) error {
	if entry == nil || entry.ProjectRoot == "" {
		return errors.New("startup cache entry requires project root")
	}
	cacheDir := filepath.Join(entry.ProjectRoot, filepath.FromSlash(subdir))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(cacheDir, stateGobFile)
	tmp, err := os.CreateTemp(cacheDir, "startup-*.gob")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	writeErr := func() error {
		defer os.Remove(tmpPath)
		enc := gob.NewEncoder(tmp)
		if err := enc.Encode(entry); err != nil {
			_ = tmp.Close()
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
