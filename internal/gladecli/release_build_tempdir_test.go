package gladecli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseBuildUsesTMPDIRAndCleansWorkdirAfterEarlyFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX shell and mktemp")
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(repoRoot, "scripts", "release-build.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("stat release builder: %v", err)
	}

	realMktemp, err := exec.LookPath("mktemp")
	if err != nil {
		t.Fatalf("find system mktemp: %v", err)
	}

	root := t.TempDir()
	tmpDir := filepath.Join(root, "owned tmp")
	fakeBin := filepath.Join(root, "bin")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fakeBin, 0o700); err != nil {
		t.Fatal(err)
	}
	workdirMarker := filepath.Join(root, "workdir-path")
	toolMarker := filepath.Join(root, "first-package-tool")

	// Model this host's bare -d behavior as an unowned default. Any explicit
	// template beneath TMPDIR is delegated to real mktemp.
	mktempShim := `#!/bin/sh
set -eu
owned_template=
for arg in "$@"; do
	case "$arg" in
		-t)
			owned_template=1
			;;
		"$TMPDIR")
			owned_template=1
			;;
		"$TMPDIR"/*)
			case "$arg" in *XXXXXX*) owned_template=1 ;; esac
			;;
	esac
done
if [ "$owned_template" != 1 ]; then
	echo "release builder did not pass a temporary template under TMPDIR" >&2
	exit 98
fi
created="$("$RELEASE_BUILD_TEST_REAL_MKTEMP" "$@")"
printf '%s\n' "$created" > "$RELEASE_BUILD_TEST_WORKDIR_MARKER"
printf '%s\n' "$created"
`
	if err := os.WriteFile(filepath.Join(fakeBin, "mktemp"), []byte(mktempShim), 0o755); err != nil {
		t.Fatal(err)
	}

	// Either dependency branch is safe: stop before npm or node can modify the
	// checkout, and prove the temporary workdir exists at that real script step.
	packageToolShim := `#!/bin/sh
set -eu
workdir="$(cat "$RELEASE_BUILD_TEST_WORKDIR_MARKER")"
case "$workdir" in
	"$TMPDIR"/*) ;;
	*) echo "builder workdir is outside TMPDIR" >&2; exit 98 ;;
esac
if [ ! -d "$workdir" ]; then
	echo "builder workdir disappeared before package preparation" >&2
	exit 98
fi
printf '%s\n' "$(basename "$0")" > "$RELEASE_BUILD_TEST_TOOL_MARKER"
exit 97
`
	for _, name := range []string{"npm", "node"} {
		if err := os.WriteFile(filepath.Join(fakeBin, name), []byte(packageToolShim), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("/bin/bash", script)
	cmd.Env = releaseBuildTestEnv(os.Environ(), map[string]string{
		"PATH":                              fakeBin + ":/usr/bin:/bin:/usr/sbin:/sbin",
		"TMPDIR":                            tmpDir,
		"DIST_DIR":                          filepath.Join(root, "dist"),
		"RELEASE_BUILD_TEST_REAL_MKTEMP":    realMktemp,
		"RELEASE_BUILD_TEST_WORKDIR_MARKER": workdirMarker,
		"RELEASE_BUILD_TEST_TOOL_MARKER":    toolMarker,
	})
	output, runErr := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 97 {
		t.Fatalf("release builder did not stop at the controlled package boundary (exit 97): err=%v\n%s", runErr, output)
	}

	tool, err := os.ReadFile(toolMarker)
	if err != nil {
		t.Fatalf("release builder never reached fake npm/node boundary: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(tool)); got != "npm" && got != "node" {
		t.Fatalf("unexpected package boundary tool %q", got)
	}
	workdirBytes, err := os.ReadFile(workdirMarker)
	if err != nil {
		t.Fatalf("release builder did not record an allocated workdir: %v\n%s", err, output)
	}
	workdir := strings.TrimSpace(string(workdirBytes))
	relative, err := filepath.Rel(tmpDir, workdir)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		t.Fatalf("workdir %q is not beneath supplied TMPDIR %q (relative=%q, err=%v)", workdir, tmpDir, relative, err)
	}
	if _, err := os.Stat(workdir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("release builder did not clean its temporary workdir: stat err=%v", err)
	}
}

func releaseBuildTestEnv(base []string, overrides map[string]string) []string {
	result := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, replaced := overrides[key]; replaced {
				continue
			}
		}
		result = append(result, entry)
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}
