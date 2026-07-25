package pluginhost

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallArchiveInstallsExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	root := t.TempDir()
	archive := filepath.Join(root, "compat.tar.gz")
	manifest := testManifestJSON("compat", "0.1.0")
	exe := []byte("#!/bin/sh\nif [ \"$1\" = \"manifest\" ]; then cat plugin.json; exit 0; fi\necho compat\n")
	checksums := testChecksums(map[string][]byte{
		"plugin.json":             manifest,
		"bin/glade-plugin-compat": exe,
	})
	writeTestArchive(t, archive, map[string]archiveFile{
		"plugin.json":             {Data: manifest, Mode: 0o644},
		"checksums.txt":           {Data: checksums, Mode: 0o644},
		"bin/glade-plugin-compat": {Data: exe, Mode: 0o755},
	})
	store := NewStore(filepath.Join(root, "home"))

	plugin, err := store.InstallArchive(context.Background(), archive)
	if err != nil {
		t.Fatal(err)
	}
	if plugin.Name != "compat" || plugin.Linked || plugin.Source != "archive:"+archive {
		t.Fatalf("unexpected plugin: %#v", plugin)
	}
	if _, err := os.Stat(plugin.Executable); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(plugin.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(exe) {
		t.Fatalf("executable contents = %q, want %q", got, exe)
	}
	info, err := os.Stat(plugin.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("executable mode = %o, want 755", info.Mode().Perm())
	}
	state, err := store.ReadInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Plugins) != 1 || state.Plugins[0].Commands[0] != "compat" {
		t.Fatalf("unexpected installed state: %#v", state)
	}
}

func TestWriteArchiveFileRejectsSymlinkEscape(t *testing.T) {
	parent := t.TempDir()
	extractionRoot := filepath.Join(parent, "extract")
	if err := os.Mkdir(extractionRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(parent, "sentinel")
	if err := os.WriteFile(sentinel, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sentinel, filepath.Join(extractionRoot, "escape")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(extractionRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if err := writeArchiveFile(root, "escape", bytes.NewReader([]byte("replacement")), 0o644); err == nil {
		t.Fatal("expected symlink escape to fail")
	}
	got, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Fatalf("outside sentinel changed to %q", got)
	}
}

func TestInstallArchiveRejectsUnsafePath(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "bad.tar.gz")
	writeTestArchive(t, archive, map[string]archiveFile{
		"../plugin.json": {Data: []byte("{}"), Mode: 0o644},
	})

	_, err := NewStore(filepath.Join(root, "home")).InstallArchive(context.Background(), archive)
	if err == nil {
		t.Fatal("expected unsafe path to fail")
	}
	if !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallArchiveRejectsBackslashPath(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "bad-backslash.tar.gz")
	writeTestArchive(t, archive, map[string]archiveFile{
		`bin\glade-plugin-compat`: {Data: []byte("#!/bin/sh\n"), Mode: 0o755},
	})

	_, err := NewStore(filepath.Join(root, "home")).InstallArchive(context.Background(), archive)
	if err == nil {
		t.Fatal("expected unsafe path to fail")
	}
	if !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallArchiveRejectsUnsafeManifestName(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "bad-name.tar.gz")
	manifest := testManifestJSON("../../owned", "0.1.0")
	exe := []byte("#!/bin/sh\n")
	writeTestArchive(t, archive, map[string]archiveFile{
		"plugin.json":                  {Data: manifest, Mode: 0o644},
		"checksums.txt":                {Data: testChecksums(map[string][]byte{"plugin.json": manifest, "bin/glade-plugin-../../owned": exe}), Mode: 0o644},
		"bin/glade-plugin-../../owned": {Data: exe, Mode: 0o755},
	})
	home := filepath.Join(root, "home")

	_, err := NewStore(home).InstallArchive(context.Background(), archive)
	if err == nil {
		t.Fatal("expected unsafe manifest name to fail")
	}
	if _, statErr := os.Stat(filepath.Join(root, "owned")); !os.IsNotExist(statErr) {
		t.Fatalf("unsafe plugin path was created, stat err=%v", statErr)
	}
}

func TestInstallArchiveRejectsChecksumMismatch(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "compat.tar.gz")
	manifest := testManifestJSON("compat", "0.1.0")
	exe := []byte("#!/bin/sh\n")
	writeTestArchive(t, archive, map[string]archiveFile{
		"plugin.json":             {Data: manifest, Mode: 0o644},
		"checksums.txt":           {Data: []byte(strings.Repeat("0", 64) + "  plugin.json\n"), Mode: 0o644},
		"bin/glade-plugin-compat": {Data: exe, Mode: 0o755},
	})

	_, err := NewStore(filepath.Join(root, "home")).InstallArchive(context.Background(), archive)
	if err == nil {
		t.Fatal("expected checksum mismatch")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallArchiveRequiresManifestAndExecutable(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "compat.tar.gz")
	manifest := testManifestJSON("compat", "0.1.0")
	writeTestArchive(t, archive, map[string]archiveFile{
		"plugin.json":   {Data: manifest, Mode: 0o644},
		"checksums.txt": {Data: testChecksums(map[string][]byte{"plugin.json": manifest}), Mode: 0o644},
	})

	_, err := NewStore(filepath.Join(root, "home")).InstallArchive(context.Background(), archive)
	if err == nil {
		t.Fatal("expected missing executable to fail")
	}
	if !strings.Contains(err.Error(), "glade-plugin-compat") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type archiveFile struct {
	Data []byte
	Mode int64
	Type byte
}

func writeTestArchive(t *testing.T, path string, files map[string]archiveFile) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	for name, file := range files {
		mode := file.Mode
		if mode == 0 {
			mode = 0o644
		}
		typeflag := file.Type
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(file.Data)), Typeflag: typeflag}); err != nil {
			t.Fatal(err)
		}
		if len(file.Data) > 0 {
			if _, err := tw.Write(file.Data); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func testManifestJSON(name, version string) []byte {
	return []byte(fmt.Sprintf(`{"apiVersion":"glade.plugin.v1","name":%q,"version":%q,"commands":[{"path":[%q],"summary":"commands"}]}`+"\n", name, version, name))
}

func testChecksums(files map[string][]byte) []byte {
	var out strings.Builder
	for name, data := range files {
		fmt.Fprintf(&out, "%x  %s\n", sha256.Sum256(data), name)
	}
	return []byte(out.String())
}
