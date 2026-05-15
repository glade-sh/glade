package watch

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitChangesSince returns watchable file changes between ref and the current
// working tree. It includes staged and unstaged tracked changes plus untracked
// watchable files.
func GitChangesSince(root, ref string) ([]Change, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, nil
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	out, err := exec.Command("git", "-C", absRoot, "diff", "--name-status", "--find-renames", ref, "--").Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-status %s: %w", ref, err)
	}
	changes := parseGitNameStatus(absRoot, out)
	untracked, err := exec.Command("git", "-C", absRoot, "ls-files", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files --others: %w", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(untracked))
	for scanner.Scan() {
		if change, ok := gitChangeFromPath(absRoot, ChangeAdded, scanner.Text()); ok {
			changes = append(changes, change)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return changes, nil
}

func parseGitNameStatus(root string, data []byte) []Change {
	var changes []Change
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		status := fields[0]
		path := fields[len(fields)-1]
		op := ChangeModified
		switch {
		case strings.HasPrefix(status, "A"):
			op = ChangeAdded
		case strings.HasPrefix(status, "D"):
			op = ChangeDeleted
		case strings.HasPrefix(status, "R"):
			op = ChangeModified
		}
		if change, ok := gitChangeFromPath(root, op, path); ok {
			changes = append(changes, change)
		}
	}
	return changes
}

func gitChangeFromPath(root string, op ChangeOp, rel string) (Change, bool) {
	path := filepath.Join(root, filepath.FromSlash(strings.TrimSpace(rel)))
	classification := ClassifyPath(path)
	if !classification.Watchable {
		return Change{}, false
	}
	change := Change{
		Path:       path,
		Op:         op,
		Kind:       classification.Kind,
		Name:       classification.Name,
		ObjectName: classification.ObjectName,
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		metadata := FileMetadata{ModTimeUnixNano: info.ModTime().UnixNano(), Size: info.Size()}
		change.After = &metadata
	}
	return change, true
}
