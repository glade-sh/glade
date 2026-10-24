package gladehome

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// RepoRoot locates a glade source checkout (go.mod + LWC toolchain). Use this for
// repository files such as testdata/.
func RepoRoot() (string, error) {
	if root, ok, err := findToolchainRoot(false); err != nil {
		return "", err
	} else if ok && hasGoMod(root) {
		return root, nil
	}
	return "", fmt.Errorf("could not find glade source checkout")
}

// SourceRoot locates a glade source checkout without requiring the local LWC
// compiler dependencies. Use this for checked-in source assets.
func SourceRoot() (string, error) {
	candidates := candidateRoots(false)
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		if hasGoMod(candidate) && hasRuntimeSource(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find glade source checkout")
}

// Root returns the active LWC toolchain root (typically ~/.local/share/glade).
func Root() (string, error) {
	root, _, err := toolchainRoot()
	return root, err
}

// toolchainRoot waits for any installer writing the managed share. Explicit
// checkouts outside that share need neither the lock nor a writable share.
func toolchainRoot() (string, string, error) {
	root, detail, err := findActiveToolchainRoot()
	if err != nil || !sameDirectory(root, UserShareDir()) {
		return root, detail, err
	}
	installMu.Lock()
	defer installMu.Unlock()
	lock, err := readInstallLock()
	if err != nil {
		return "", "", err
	}
	if lock != nil {
		defer lock.Close()
	}
	// Validation alone cannot establish completion while an installer is copying.
	return findActiveToolchainRoot()
}

func findActiveToolchainRoot() (string, string, error) {
	if root, ok := explicitToolchainRoot(); ok {
		return root, "ok (explicit)", nil
	}
	if root, ok := userShareRoot(); ok {
		return root, "ok (global)", nil
	}
	if root, ok, err := findToolchainRoot(false); err != nil {
		return "", "", err
	} else if ok {
		return root, "ok (discovered)", nil
	}
	return "", "", fmt.Errorf("glade LWC toolchain not found; run `glade toolchain install` from a glade checkout or set GLADE_HOME")
}

// installMu serializes installers within this process before taking the file lock.
var installMu sync.Mutex

// EnsureRoot returns the user-global toolchain directory, installing from a discovered
// source checkout when needed.
func EnsureRoot() (string, error) {
	if root, ok := explicitToolchainRoot(); ok && !sameDirectory(root, UserShareDir()) {
		return root, nil
	}
	installMu.Lock()
	defer installMu.Unlock()
	if root, ok, err := readyInstalledRoot(); err != nil {
		return "", err
	} else if ok {
		return root, nil
	}
	lock, err := lockInstall()
	if err != nil {
		return "", err
	}
	defer lock.Close()
	// Another process may have installed between verification and the write lock.
	if root, ok := explicitToolchainRoot(); ok {
		return root, nil
	}
	if root, ok := userShareRoot(); ok {
		return root, nil
	}
	src, err := findInstallSource()
	if err != nil {
		return "", err
	}
	if err := installFrom(src); err != nil {
		return "", err
	}
	root, ok := userShareRoot()
	if !ok {
		return "", fmt.Errorf("installed glade toolchain is incomplete at %s", UserShareDir())
	}
	return root, nil
}

// readyInstalledRoot verifies a completed home while holding an existing read
// lock when available. It releases that lock before EnsureRoot becomes a writer.
func readyInstalledRoot() (string, bool, error) {
	lock, err := readInstallLock()
	if err != nil {
		return "", false, err
	}
	if lock != nil {
		defer lock.Close()
	}
	if root, ok := explicitToolchainRoot(); ok {
		return root, true, nil
	}
	root, ok := userShareRoot()
	return root, ok, nil
}

// sameDirectory compares filesystem identity so relative paths, symlinks and
// case aliases of the managed share all use the install lock.
func sameDirectory(a, b string) bool {
	stat := func(path string) (os.FileInfo, error) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return nil, err
		}
		return os.Stat(resolved)
	}
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return false
	}
	left, err := stat(a)
	if err != nil || !left.IsDir() {
		return false
	}
	right, err := stat(b)
	return err == nil && right.IsDir() && os.SameFile(left, right)
}

func explicitToolchainRoot() (string, bool) {
	for _, value := range []string{os.Getenv("GLADE_HOME"), os.Getenv("GLADE_ROOT")} {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if root, ok := validateRoot(value); ok {
			return root, true
		}
	}
	return "", false
}

// LWCToolchainDir is the directory passed to the Node compile script (third_party/lwc).
func LWCToolchainDir() (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "third_party", "lwc"), nil
}

// ShimsDir holds browser wire-adapter modules served at /lightning/shims/core/.
func ShimsDir() (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "lwcruntime", "src", "shims"), nil
}

// RuntimeAssetDir holds browser runtime modules served at /lightning/runtime/.
func RuntimeAssetDir(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch name {
	case "experience", "lightning", "shell", "shims", "slds":
	default:
		return "", fmt.Errorf("unknown LWC runtime asset directory %q", name)
	}
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "lwcruntime", "src", name), nil
}

// UserShareDir is the default global install location (~/.local/share/glade).
func UserShareDir() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
		return filepath.Join(xdg, "glade")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "glade-share")
	}
	return filepath.Join(home, ".local", "share", "glade")
}

func userShareRoot() (string, bool) {
	return validateRoot(UserShareDir())
}

func findToolchainRoot(includeUserShare bool) (string, bool, error) {
	candidates := candidateRoots(includeUserShare)
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		if root, ok := validateRoot(candidate); ok {
			return root, true, nil
		}
	}
	return "", false, nil
}

func candidateRoots(includeUserShare bool) []string {
	var out []string
	if env := strings.TrimSpace(os.Getenv("GLADE_HOME")); env != "" {
		out = append(out, env)
	}
	if env := strings.TrimSpace(os.Getenv("GLADE_ROOT")); env != "" {
		out = append(out, env)
	}
	if includeUserShare {
		out = append(out, UserShareDir())
	}
	if exe, err := os.Executable(); err == nil {
		exe, err = filepath.EvalSymlinks(exe)
		if err == nil {
			exeDir := filepath.Dir(exe)
			out = append(out,
				filepath.Join(exeDir, "share", "glade"),
				filepath.Join(exeDir, "..", "share", "glade"),
				exeDir,
			)
			out = append(out, walkAncestors(exeDir)...)
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		out = append(out, walkAncestors(cwd)...)
	}
	return out
}

func hasGoMod(root string) bool {
	_, err := os.Stat(filepath.Join(root, "go.mod"))
	return err == nil
}

func hasRuntimeSource(root string) bool {
	for _, required := range []string{
		filepath.Join(root, "lwcruntime", "src", "shell", "app.mjs"),
		filepath.Join(root, "lwcruntime", "src", "shims", "wire-adapter.mjs"),
		filepath.Join(root, "lwcruntime", "src", "slds", "slds-loader.mjs"),
	} {
		if _, err := os.Stat(required); err != nil {
			return false
		}
	}
	return true
}

func walkAncestors(start string) []string {
	var out []string
	dir := start
	for {
		out = append(out, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return out
}

func validateRoot(root string) (string, bool) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", false
	}
	compileScript := filepath.Join(root, "third_party", "lwc", "compile.mjs")
	if _, err := os.Stat(compileScript); err != nil {
		return "", false
	}
	vendor := filepath.Join(root, "third_party", "lwc", "node_modules", "@lwc", "compiler")
	if _, err := os.Stat(vendor); err != nil {
		return "", false
	}
	for _, required := range []string{
		filepath.Join(root, "lwcruntime", "src", "shims", "wire-adapter.mjs"),
		filepath.Join(root, "lwcruntime", "src", "shims", "lds-cache.mjs"),
		filepath.Join(root, "lwcruntime", "src", "shell", "app.mjs"),
		filepath.Join(root, "lwcruntime", "src", "slds", "slds-loader.mjs"),
		filepath.Join(root, "lwcruntime", "src", "lightning", "button.mjs"),
	} {
		if _, err := os.Stat(required); err != nil {
			return "", false
		}
	}
	return filepath.Clean(root), true
}

// ToolchainStatus describes whether the active LWC toolchain is ready.
func ToolchainStatus() (path string, ok bool, detail string) {
	root, detail, err := toolchainRoot()
	if err == nil {
		return root, true, detail
	}
	return UserShareDir(), false, "missing; reinstall the release or install from a Glade source checkout"
}
