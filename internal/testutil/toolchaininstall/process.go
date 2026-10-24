// Package toolchaininstall exercises installs in separate processes against an
// owned toolchain fixture and a cold user share.
package toolchaininstall

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const helperMode = "GLADE_TEST_INSTALL_HELPER"

// ColdShare runs every pair of routes in independent copies of the test binary.
// The first process pauses immediately before copying the compiler. Without
// serialization, the second installs into that destination before the first
// resumes, making cp nest the compiler tree inside the existing directory.
func ColdShare(t *testing.T, routes []string, install func(string) error) {
	t.Helper()
	shareInstall(t, routes, install, false)
}

// PendingEditor pauses after the required runtime assets are visible but before
// the editor bundle has been installed. EnsureRoot must wait for the installer.
func PendingEditor(t *testing.T, install func(string) error) {
	t.Helper()
	shareInstall(t, []string{"EnsureRoot"}, install, true)
}

// PendingConsumers starts one installer, then exercises each consumer while the
// runtime is visible and the editor is still pending. The callback must install
// from cwd for the "install" route and invoke the named consumer otherwise.
func PendingConsumers(t *testing.T, routes []string, install func(string) error) {
	t.Helper()
	shareInstallRoutes(t, []string{"install"}, routes, install, true, false)
}

// PendingIndependentConsumer proves an entry point that does not resolve the
// toolchain can finish while an installation is still paused.
func PendingIndependentConsumer(t *testing.T, route string, run func(string) error) {
	t.Helper()
	shareInstallRoutes(t, []string{"install"}, []string{route}, run, true, true)
}

func shareInstall(t *testing.T, routes []string, install func(string) error, pendingEditor bool) {
	t.Helper()
	shareInstallRoutes(t, routes, routes, install, pendingEditor, false)
}

func shareInstallRoutes(t *testing.T, firstRoutes, secondRoutes []string, install func(string) error, pendingEditor, independent bool) {
	t.Helper()
	switch os.Getenv(helperMode) {
	case "install":
		if err := install(os.Getenv("GLADE_TEST_INSTALL_ROUTE")); err != nil {
			t.Fatal(err)
		}
		if os.Getenv("GLADE_TEST_INSTALL_INDEPENDENT") == "true" {
			return
		}
		if _, err := os.Stat(filepath.Join(os.Getenv("XDG_DATA_HOME"), "glade", "editor", "vscode-glade.vsix")); err != nil {
			t.Fatalf("install returned before the editor bundle was ready: %v", err)
		}
		return
	case "copy":
		copyHelper(t)
		return
	}
	if runtime.GOOS == "windows" {
		t.Skip("the copy interleaving uses the Unix cp fast path")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cp, err := exec.LookPath("cp")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	src := filepath.Join(root, "source")
	files := sourceFixture(t, src)
	bin := filepath.Join(root, "bin")
	writeFile(t, filepath.Join(bin, "cp"), `#!/bin/sh
export GLADE_TEST_INSTALL_HELPER=copy
exec "$GLADE_TEST_INSTALL_EXE" "-test.run=$GLADE_TEST_INSTALL_RUN" -- "$@"
`, 0o755)
	for _, firstRoute := range firstRoutes {
		for _, secondRoute := range secondRoutes {
			t.Run(firstRoute+"_then_"+secondRoute, func(t *testing.T) {
				// Keep the share below the source fixture so a plain relative
				// alias can be selected without changing the consumer's cwd.
				share, err := os.MkdirTemp(src, "share-")
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasSuffix(secondRoute, "_case") {
					dir := filepath.Join(share, "glade")
					if err := os.MkdirAll(dir, 0o755); err != nil {
						t.Fatal(err)
					}
					info, err := os.Stat(dir)
					if err != nil {
						t.Fatal(err)
					}
					variant, err := os.Stat(filepath.Join(share, "GLADE"))
					if os.IsNotExist(err) || err == nil && !os.SameFile(info, variant) {
						t.Skip("case-variant alias requires a case-insensitive filesystem; this filesystem is case-sensitive")
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				control := t.TempDir()
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				start := func(route string, hold bool) (<-chan error, *bytes.Buffer) {
					cmd := exec.CommandContext(ctx, exe, "-test.run=^"+strings.Split(t.Name(), "/")[0]+"$")
					cmd.Dir = src
					cmd.Env = append(os.Environ(),
						"GLADE_HOME=", "GLADE_ROOT=", "XDG_DATA_HOME="+share,
						helperMode+"=install", "GLADE_TEST_INSTALL_ROUTE="+route,
						"GLADE_TEST_INSTALL_EXE="+exe, "GLADE_TEST_INSTALL_RUN=^"+strings.Split(t.Name(), "/")[0]+"$",
						"GLADE_TEST_INSTALL_CP="+cp, "GLADE_TEST_INSTALL_CONTROL="+control,
						fmt.Sprintf("GLADE_TEST_INSTALL_HOLD=%t", hold),
						fmt.Sprintf("GLADE_TEST_INSTALL_INDEPENDENT=%t", independent && !hold),
						fmt.Sprintf("GLADE_TEST_INSTALL_PENDING_EDITOR=%t", pendingEditor),
						"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
					var output bytes.Buffer
					cmd.Stdout, cmd.Stderr = &output, &output
					if err := cmd.Start(); err != nil {
						t.Fatal(err)
					}
					done := make(chan error, 1)
					go func() { done <- cmd.Wait() }()
					return done, &output
				}
				first, firstOutput := start(firstRoute, true)
				waitFile(t, ctx, filepath.Join(control, "copy-ready"))
				second, secondOutput := start(secondRoute, false)
				// A correct installer waits for the first process. An unguarded
				// installer can finish; release the first copy in either case.
				var secondErr error
				secondFinished := false
				select {
				case secondErr = <-second:
					secondFinished = true
				case <-time.After(func() time.Duration {
					if independent {
						return 3 * time.Second
					}
					return time.Second
				}()):
				}
				if independent && !secondFinished {
					t.Errorf("independent route %s blocked on the toolchain install", secondRoute)
				}
				if pendingEditor && !independent && secondFinished {
					t.Errorf("consumer %s returned while the installation was still pending", secondRoute)
				}
				writeFile(t, filepath.Join(control, "copy-release"), "", 0o644)
				if err := <-first; err != nil {
					t.Errorf("first %s: %v\n%s", firstRoute, err, firstOutput)
				}
				if !secondFinished {
					secondErr = <-second
				}
				if secondErr != nil {
					t.Errorf("second %s: %v\n%s", secondRoute, secondErr, secondOutput)
				}
				dst := filepath.Join(share, "glade")
				if _, err := os.Stat(filepath.Join(dst, "third_party", "lwc", "lwc")); !os.IsNotExist(err) {
					t.Error("concurrent installs nested the compiler tree inside itself")
				}
				if secondRoute == "EnsureRoot" {
					if _, err := os.Stat(filepath.Join(control, "copy-second")); !os.IsNotExist(err) {
						t.Error("EnsureRoot reinstalled a toolchain completed by another process")
					}
				}
				for _, rel := range files[1:] {
					installed := rel
					if strings.HasSuffix(rel, ".vsix") {
						installed = "editor/vscode-glade.vsix"
					}
					got, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(installed)))
					want, readErr := os.ReadFile(filepath.Join(src, filepath.FromSlash(rel)))
					if readErr != nil {
						t.Fatal(readErr)
					}
					if err != nil || string(got) != string(want) {
						t.Errorf("installed %s = %q, %v; want %q", installed, got, err, want)
					}
				}
			})
		}
	}
}

func copyHelper(t *testing.T) {
	t.Helper()
	var args []string
	for i, arg := range os.Args {
		if arg == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) != 3 || args[0] != "-R" {
		t.Fatalf("unexpected cp arguments: %q", args)
	}
	copyTree := func() {
		if output, err := exec.Command(os.Getenv("GLADE_TEST_INSTALL_CP"), args...).CombinedOutput(); err != nil {
			t.Fatalf("cp: %v\n%s", err, output)
		}
	}
	pendingEditor := os.Getenv("GLADE_TEST_INSTALL_PENDING_EDITOR") == "true"
	if pendingEditor {
		copyTree()
	}
	if (!pendingEditor && filepath.Base(args[1]) == "lwc") || (pendingEditor && filepath.Base(args[1]) == "slds") {
		control := os.Getenv("GLADE_TEST_INSTALL_CONTROL")
		if os.Getenv("GLADE_TEST_INSTALL_HOLD") == "true" {
			writeFile(t, filepath.Join(control, "copy-ready"), "", 0o644)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			waitFile(t, ctx, filepath.Join(control, "copy-release"))
		} else {
			writeFile(t, filepath.Join(control, "copy-second"), "", 0o644)
		}
	}
	if !pendingEditor {
		copyTree()
	}
}

func waitFile(t *testing.T, ctx context.Context, path string) {
	t.Helper()
	for {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for %s: %v", filepath.Base(path), ctx.Err())
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// SelectShare selects the pending user share through the named environment route.
// "default" leaves both overrides unset. Suffixes select aliases of the same
// directory: relative, parent (../), symlink, or case variant.
func SelectShare(t *testing.T, selector string) {
	t.Helper()
	if selector == "default" {
		return
	}
	share := filepath.Join(os.Getenv("XDG_DATA_HOME"), "glade")
	env := selector
	for _, suffix := range []string{"_relative", "_parent", "_symlink", "_case"} {
		if !strings.HasSuffix(selector, suffix) {
			continue
		}
		env = strings.TrimSuffix(selector, suffix)
		switch suffix {
		case "_relative":
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			share, err = filepath.Rel(cwd, share)
			if err != nil {
				t.Fatal(err)
			}
		case "_parent":
			child := filepath.Join(filepath.Dir(share), "child")
			if err := os.MkdirAll(child, 0o755); err != nil {
				t.Fatal(err)
			}
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			relative, err := filepath.Rel(cwd, child)
			if err != nil {
				t.Fatal(err)
			}
			share = relative + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "glade"
		case "_symlink":
			alias := filepath.Join(t.TempDir(), "share-link")
			if err := os.Symlink(share, alias); err != nil {
				t.Fatal(err)
			}
			share = alias
		case "_case":
			share = filepath.Join(filepath.Dir(share), "GLADE")
		}
		break
	}
	t.Setenv(env, share)
}
