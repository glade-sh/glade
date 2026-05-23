package runartifact

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Run struct {
	ID        string    `json:"id"`
	Dir       string    `json:"dir"`
	CreatedAt time.Time `json:"createdAt"`
}

type Latest struct {
	RunID       string `json:"runId"`
	RunDir      string `json:"runDir"`
	SummaryPath string `json:"summaryPath,omitempty"`
	ResultsPath string `json:"resultsPath,omitempty"`
}

func Open(root, id string, now time.Time) (Run, error) {
	if strings.TrimSpace(root) == "" {
		return Run{}, errors.New("run artifact root is required")
	}
	if strings.TrimSpace(id) == "" {
		id = now.UTC().Format("20060102-150405")
	}
	if !safeID(id) {
		return Run{}, fmt.Errorf("unsafe run ID %q", id)
	}
	run := Run{
		ID:        id,
		Dir:       filepath.Join(root, id),
		CreatedAt: now.UTC(),
	}
	if err := os.MkdirAll(run.Dir, 0o755); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (r Run) Path(name string) string {
	return filepath.Join(r.Dir, name)
}

func (r Run) WriteJSON(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(r.Path(name), data, 0o644)
}

func (r Run) WriteText(name, text string) error {
	return os.WriteFile(r.Path(name), []byte(text), 0o644)
}

func (r Run) WriteLatest(root string, latest Latest) error {
	if latest.RunID == "" {
		latest.RunID = r.ID
	}
	if latest.RunDir == "" {
		latest.RunDir = r.Dir
	}
	data, err := json.MarshalIndent(latest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(root, "latest.json"), data, 0o644)
}

func Clean(root string, keep int) (int, error) {
	if keep < 0 {
		return 0, errors.New("keep must be non-negative")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && safeID(entry.Name()) {
			dirs = append(dirs, entry.Name())
		}
	}
	sort.Strings(dirs)
	removeCount := len(dirs) - keep
	if removeCount <= 0 {
		return 0, nil
	}
	for _, id := range dirs[:removeCount] {
		if err := os.RemoveAll(filepath.Join(root, id)); err != nil {
			return 0, err
		}
	}
	return removeCount, nil
}

func safeID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	if strings.ContainsAny(id, `/\`) {
		return false
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}
