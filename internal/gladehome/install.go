package gladehome

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// InstallFrom copies the LWC toolchain from a glade checkout into UserShareDir().
func InstallFrom(src string) error {
	src, ok := validateRoot(src)
	if !ok {
		return fmt.Errorf("source %q is not a glade checkout with an installed LWC toolchain (run npm install in third_party/lwc)", src)
	}
	dst := UserShareDir()
	if samePath(src, dst) {
		return fmt.Errorf("source and destination are the same (%s); unset GLADE_HOME or run install from a glade source checkout", dst)
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
	for _, pair := range pairs {
		from := filepath.Join(src, filepath.FromSlash(pair.rel))
		to := filepath.Join(dst, filepath.FromSlash(pair.rel))
		if samePath(from, to) {
			continue
		}
		if err := os.RemoveAll(to); err != nil {
			return fmt.Errorf("reset %s: %w", to, err)
		}
		if err := copyTree(from, to); err != nil {
			return fmt.Errorf("copy %s: %w", pair.rel, err)
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
	share := filepath.Clean(UserShareDir())
	candidates := candidateRoots(false)
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
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fileMode(entry))
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
