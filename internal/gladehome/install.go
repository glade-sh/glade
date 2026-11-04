package gladehome

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// InstallFrom copies source-built toolchain assets from a glade checkout into UserShareDir().
func InstallFrom(src string) error {
	installMu.Lock()
	defer installMu.Unlock()
	lock, err := lockInstall()
	if err != nil {
		return err
	}
	defer lock.Close()
	return installFrom(src, lock)
}

func installFrom(src string, lock *os.File) error {
	if err := recoverExclusive(lock); err != nil {
		return fmt.Errorf("recover toolchain install: %w", err)
	}
	if err := cleanupAbandonedSourceStages(UserShareDir()); err != nil {
		return errors.Join(fmt.Errorf("clean source toolchain staging: %w", err), writeInstallMarker(lock, 'P'))
	}
	src, ok := validateRoot(src)
	if !ok {
		return fmt.Errorf("source %q is not a glade checkout with an installed LWC toolchain (run npm install in third_party/lwc)", src)
	}
	dst := UserShareDir()
	if samePath(src, dst) {
		return fmt.Errorf("source and destination are the same (%s); unset GLADE_HOME or run install from a glade source checkout", dst)
	}
	// Reject unprepared current assets before changing the installed toolchain.
	// This check does not patch dependencies or make another dependency copy.
	if err := verifyLWCToolchainPreparation(src, func(script, toolchain string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "node", script, "--check", toolchain)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	}); err != nil {
		return err
	}
	pairs := []struct {
		rel string
	}{
		{rel: "third_party/lwc"},
		{rel: "lwcruntime/src/experience"},
		{rel: "lwcruntime/src/lightning"},
		{rel: "lwcruntime/src/shell"},
		{rel: "lwcruntime/src/shims"},
		{rel: "lwcruntime/src/slds"},
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("create toolchain destination: %w", err)
	}
	stage, err := os.MkdirTemp(dst, ".toolchain-stage-")
	if err != nil {
		return fmt.Errorf("stage toolchain: %w", err)
	}
	keepStage := false
	defer func() {
		if !keepStage {
			_ = os.RemoveAll(stage)
		}
	}()
	var staged []string
	for _, pair := range pairs {
		from := filepath.Join(src, filepath.FromSlash(pair.rel))
		to := filepath.Join(dst, filepath.FromSlash(pair.rel))
		if samePath(from, to) {
			continue
		}
		if err := copyTree(from, filepath.Join(stage, "new", filepath.FromSlash(pair.rel))); err != nil {
			return fmt.Errorf("copy %s: %w", pair.rel, err)
		}
		staged = append(staged, pair.rel)
	}
	if err := installEditorVSIX(src, filepath.Join(stage, "new")); err != nil {
		return err
	}
	if _, err := os.Lstat(filepath.Join(stage, "new", "editor", "vscode-glade.vsix")); err == nil {
		staged = append(staged, "editor/vscode-glade.vsix")
	} else if !os.IsNotExist(err) {
		return err
	}
	if len(staged) == 0 {
		return nil
	}
	journal := make([]sourceInstallEntry, 0, len(staged))
	for _, rel := range staged {
		_, err := os.Lstat(filepath.Join(dst, filepath.FromSlash(rel)))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		journal = append(journal, sourceInstallEntry{Rel: rel, HadOld: err == nil})
	}
	journalData, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "journal.json"), journalData, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "started"), nil, 0o600); err != nil {
		return err
	}
	if err := writeInstallMarker(lock, 'P'); err != nil {
		return err
	}
	type replacement struct {
		target string
		backup string
		hadOld bool
	}
	var applied []replacement
	rollback := func() error {
		var result error
		for i := len(applied) - 1; i >= 0; i-- {
			entry := applied[i]
			if err := os.RemoveAll(entry.target); err != nil {
				result = errors.Join(result, err)
				continue
			}
			if entry.hadOld {
				result = errors.Join(result, os.Rename(entry.backup, entry.target))
			}
		}
		return result
	}
	fail := func(cause error) error {
		if err := rollback(); err != nil {
			keepStage = true
			return errors.Join(cause, fmt.Errorf("rollback failed; previous files retained under %s: %w", stage, err))
		}
		if err := writeInstallMarker(lock, 0); err != nil {
			keepStage = true
			return errors.Join(cause, fmt.Errorf("clear toolchain recovery marker: %w", err))
		}
		return cause
	}
	for _, item := range journal {
		rel := item.Rel
		target := filepath.Join(dst, filepath.FromSlash(rel))
		backup := filepath.Join(stage, "old", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fail(err)
		}
		entry := replacement{target: target, backup: backup}
		if item.HadOld {
			if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
				return fail(err)
			}
			if err := os.Rename(target, backup); err != nil {
				return fail(err)
			}
			entry.hadOld = true
		}
		applied = append(applied, entry)
		if err := os.Rename(filepath.Join(stage, "new", filepath.FromSlash(rel)), target); err != nil {
			return fail(fmt.Errorf("install %s: %w", rel, err))
		}
	}
	if err := os.WriteFile(filepath.Join(stage, "commit.tmp"), nil, 0o600); err != nil {
		return fail(err)
	}
	if err := os.Rename(filepath.Join(stage, "commit.tmp"), filepath.Join(stage, "committed")); err != nil {
		return fail(err)
	}
	if err := os.RemoveAll(stage); err != nil {
		keepStage = true
		return err
	}
	if err := writeInstallMarker(lock, 0); err != nil {
		keepStage = true
		return err
	}
	return nil
}

// lwcPreparationScripts lists the current @lwc/shared preparation script
// first, then the earlier API 67-only name that older checkouts still carry.
var lwcPreparationScripts = []string{"prepare-lwc-shared.mjs", "apply-lwc-shared-api67.mjs"}

func verifyLWCToolchainPreparation(src string, check func(string, string) error) error {
	toolchain := filepath.Join(src, "third_party", "lwc")
	scriptPath := func(name string) string { return filepath.Join(toolchain, "scripts", name) }
	exists := func(path string) (bool, error) {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, fmt.Errorf("inspect LWC preparation: %w", err)
		}
		return true, nil
	}
	verify := func(script string) error {
		if err := check(script, toolchain); err != nil {
			return fmt.Errorf("LWC toolchain is not prepared; run its pinned npm preparation before install: %w", err)
		}
		return nil
	}
	data, readErr := os.ReadFile(filepath.Join(toolchain, "package.json"))
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("read LWC package: %w", readErr)
	}
	if readErr == nil {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		if err := json.Unmarshal(data, &pkg); err != nil {
			return fmt.Errorf("read LWC package: %w", err)
		}
		// A declared preparation hook is authoritative: verify with that script
		// and never fall back to another script left in the tree.
		for _, name := range lwcPreparationScripts {
			if !strings.Contains(pkg.Scripts["postinstall"], name) {
				continue
			}
			script := scriptPath(name)
			ok, err := exists(script)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("LWC preparation script missing: %s", script)
			}
			return verify(script)
		}
	}
	// Without a declared hook, verify with whichever known script the tree has.
	// Older toolchains with neither remain installable; this grants no API 67
	// or 68 support.
	for _, name := range lwcPreparationScripts {
		script := scriptPath(name)
		ok, err := exists(script)
		if err != nil {
			return err
		}
		if ok {
			return verify(script)
		}
	}
	return nil
}

// InstallFromCWD installs from the nearest glade checkout found by walking up from cwd.
func InstallFromCWD() error {
	src, err := findInstallSource()
	if err != nil {
		return err
	}
	return InstallFrom(src)
}

func findInstallSource() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("find current checkout: %w", err)
	}
	return findInstallSourceCandidates(walkAncestors(cwd))
}

func findInstallSourceCandidates(candidates []string) (string, error) {
	share := filepath.Clean(UserShareDir())
	seen := make(map[string]bool, len(candidates))
	var fallback string
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if candidate == "" || seen[candidate] || candidate == share {
			continue
		}
		seen[candidate] = true
		root, ok := validateRoot(candidate)
		if !ok {
			continue
		}
		if hasGoMod(root) {
			return root, nil
		}
		if fallback == "" {
			fallback = root
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("no glade checkout with LWC toolchain found; run from a glade repo or use --from <path>")
}

func installEditorVSIX(src, dst string) error {
	pattern := filepath.Join(src, "contrib", "vscode-glade", "dist", "vscode-glade-*.vsix")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("find editor vsix: %w", err)
	}
	if len(matches) == 0 {
		return nil
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	to := filepath.Join(dst, "editor", "vscode-glade.vsix")
	if samePath(matches[0], to) {
		return nil
	}
	if err := copyFilePath(matches[0], to, 0o644); err != nil {
		return fmt.Errorf("copy editor vscode-glade.vsix: %w", err)
	}
	return nil
}

func samePath(a, b string) bool {
	a = filepath.Clean(strings.TrimSpace(a))
	b = filepath.Clean(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	ar, errA := filepath.EvalSymlinks(a)
	br, errB := filepath.EvalSymlinks(b)
	if errA == nil && errB == nil {
		return ar == br
	}
	return false
}

func copyTree(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := exec.Command("cp", "-R", src, dst).Run(); err == nil {
			return nil
		}
	}
	return copyTreeWalk(src, dst)
}

func copyTreeWalk(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		return copyFile(path, target, entry)
	})
}

func copyFile(src, dst string, entry fs.DirEntry) error {
	return copyFilePath(src, dst, fileMode(entry))
}

func copyFilePath(src, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func fileMode(entry fs.DirEntry) fs.FileMode {
	if info, err := entry.Info(); err == nil {
		return info.Mode().Perm()
	}
	return 0o644
}
