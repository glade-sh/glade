package playground

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type CacheKey struct {
	WorkspaceHash    string
	AnonymousBody    string
	SeedHash         string
	ProjectRoot      string
	LimitMode        string
	RunMode          string
	Version          string
	SourceAPIVersion string
}

func (k CacheKey) String() string {
	h := sha256.New()
	for _, part := range []string{k.WorkspaceHash, k.AnonymousBody, k.SeedHash, k.ProjectRoot, k.LimitMode, k.RunMode, k.Version, k.SourceAPIVersion} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

type ResultCache struct {
	root string
}

func NewResultCache(root string) *ResultCache {
	return &ResultCache{root: root}
}

func (c *ResultCache) Store(key string, result RunResult) error {
	if c == nil || c.root == "" {
		return nil
	}
	result.CacheKey = key
	path := c.pathForKey(key)
	if err := writeJSONFile(path, result); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(c.root, "runs", "latest.json"), result)
}

func (c *ResultCache) Load(key string) (RunResult, bool, error) {
	if c == nil || c.root == "" {
		return RunResult{}, false, nil
	}
	data, err := os.ReadFile(c.pathForKey(key))
	if errors.Is(err, os.ErrNotExist) {
		return RunResult{}, false, nil
	}
	if err != nil {
		return RunResult{}, false, err
	}
	var result RunResult
	if err := json.Unmarshal(data, &result); err != nil {
		return RunResult{}, false, err
	}
	return result, true, nil
}

func (c *ResultCache) Latest() (RunResult, bool, error) {
	if c == nil || c.root == "" {
		return RunResult{}, false, nil
	}
	data, err := os.ReadFile(filepath.Join(c.root, "runs", "latest.json"))
	if errors.Is(err, os.ErrNotExist) {
		return RunResult{}, false, nil
	}
	if err != nil {
		return RunResult{}, false, err
	}
	var result RunResult
	if err := json.Unmarshal(data, &result); err != nil {
		return RunResult{}, false, err
	}
	return result, true, nil
}

func (c *ResultCache) ClearLatest() error {
	if c == nil || c.root == "" {
		return nil
	}
	err := os.Remove(filepath.Join(c.root, "runs", "latest.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (c *ResultCache) pathForKey(key string) string {
	key = strings.TrimPrefix(key, "sha256:")
	return filepath.Join(c.root, "cache", key+".json")
}
