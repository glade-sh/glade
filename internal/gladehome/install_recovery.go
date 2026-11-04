package gladehome

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type sourceInstallEntry struct {
	Rel    string `json:"rel"`
	HadOld bool   `json:"hadOld"`
}

// cleanupAbandonedSourceStages requires the exclusive lock and an empty
// recovery marker. Backups are never removed without a committed journal.
func cleanupAbandonedSourceStages(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), ".toolchain-stage-") {
			continue
		}
		stage := filepath.Join(dir, entry.Name())
		_, committedErr := os.Stat(filepath.Join(stage, "committed"))
		_, oldErr := os.Lstat(filepath.Join(stage, "old"))
		if committedErr != nil && !os.IsNotExist(committedErr) {
			return committedErr
		}
		if oldErr != nil && !os.IsNotExist(oldErr) {
			return oldErr
		}
		if committedErr != nil && oldErr == nil {
			return fmt.Errorf("source staging has retained backups without a recovery marker: %s", stage)
		}
		if err := os.RemoveAll(stage); err != nil {
			return err
		}
	}
	return nil
}

// recoverSourceInstalls requires the exclusive install lock. Uncommitted swaps
// are restored; committed generations only need their staging directory removed.
func recoverSourceInstalls(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	allowed := map[string]bool{"third_party/lwc": true, "lwcruntime/src/experience": true, "lwcruntime/src/lightning": true, "lwcruntime/src/shell": true, "lwcruntime/src/shims": true, "lwcruntime/src/slds": true, "editor/vscode-glade.vsix": true}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), ".toolchain-stage-") {
			continue
		}
		stage := filepath.Join(dir, entry.Name())
		if _, err := os.Stat(filepath.Join(stage, "started")); os.IsNotExist(err) {
			if _, oldErr := os.Lstat(filepath.Join(stage, "old")); oldErr == nil {
				return fmt.Errorf("source staging contains an unjournaled backup: %s", stage)
			} else if !os.IsNotExist(oldErr) {
				return oldErr
			}
			continue
		} else if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(stage, "committed")); err == nil {
			if err := os.RemoveAll(stage); err != nil {
				return err
			}
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		data, err := os.ReadFile(filepath.Join(stage, "journal.json"))
		if err != nil {
			return fmt.Errorf("read source install journal: %w", err)
		}
		var journal []sourceInstallEntry
		if err := json.Unmarshal(data, &journal); err != nil {
			return fmt.Errorf("parse source install journal: %w", err)
		}
		if len(journal) == 0 {
			return fmt.Errorf("empty source install journal")
		}
		seen := make(map[string]bool, len(journal))
		for _, item := range journal {
			if !allowed[item.Rel] || seen[item.Rel] {
				return fmt.Errorf("invalid source install journal entry")
			}
			seen[item.Rel] = true
		}
		for i := len(journal) - 1; i >= 0; i-- {
			item := journal[i]
			target := filepath.Join(dir, filepath.FromSlash(item.Rel))
			backup := filepath.Join(stage, "old", filepath.FromSlash(item.Rel))
			if _, err := os.Lstat(backup); err == nil {
				if err := os.RemoveAll(target); err != nil {
					return err
				}
				if err := os.Rename(backup, target); err != nil {
					return err
				}
			} else if os.IsNotExist(err) {
				if !item.HadOld {
					if err := os.RemoveAll(target); err != nil {
						return err
					}
				}
			} else {
				return err
			}
		}
		if err := os.RemoveAll(stage); err != nil {
			return err
		}
	}
	return nil
}
