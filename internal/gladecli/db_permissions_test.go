package gladecli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenDBStoreCreatesPrivateFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not available on Windows")
	}
	for _, suffix := range []string{"", "?_pragma=busy_timeout%3d5000"} {
		t.Run(suffix, func(t *testing.T) {
			root := writeDBPermissionsProject(t)
			dbPath := projectEnvDBPath(root, "dev")
			store, _, err := openDBStore(dbPath+suffix, root)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			assertPathPermissions(t, filepath.Join(root, ".glade"), 0o700)
			assertPathPermissions(t, filepath.Dir(dbPath), 0o700)
			assertPathPermissions(t, dbPath, 0o600)
			if suffix != "" {
				if _, err := os.Stat(dbPath + suffix); !os.IsNotExist(err) {
					t.Fatalf("literal DSN file should not exist: %v", err)
				}
			}
		})
	}
}

func TestOpenDBStorePreservesMemoryDSNs(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)
	for _, dsn := range []string{":memory:", ":memory:?cache=shared"} {
		t.Run(dsn, func(t *testing.T) {
			store, _, err := openDBStore(dsn, writeDBPermissionsProject(t))
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("memory DSN created filesystem entries: %v", entries)
			}
		})
	}
}

func TestOpenDBStorePreservesExistingFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not available on Windows")
	}
	root := writeDBPermissionsProject(t)
	dbPath := projectEnvDBPath(root, "dev")
	store, org, err := openDBStore(dbPath, root)
	if err != nil {
		t.Fatal(err)
	}
	org.DomainURL = "https://stored.example.test"
	if err := store.Save(org); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dbPath, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(dbPath), 0o750); err != nil {
		t.Fatal(err)
	}

	store, got, err := openDBStore(dbPath, root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if got.DomainURL != org.DomainURL {
		t.Fatalf("stored domain = %q, want %q; existing database was changed", got.DomainURL, org.DomainURL)
	}
	assertPathPermissions(t, filepath.Dir(dbPath), 0o750)
	assertPathPermissions(t, dbPath, 0o640)
}

func TestOpenDBStoreDoesNotTruncateInvalidExistingFile(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "invalid.sqlite")
	want := []byte("caller-owned file that is not a SQLite database\n")
	if err := os.WriteFile(dbPath, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if store, _, err := openDBStore(dbPath, root); err == nil {
		_ = store.Close()
		t.Fatal("invalid existing database was accepted")
	}
	got, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("existing file = %q, want %q", got, want)
	}
}

func TestWriteDebugLogCreatesPrivateFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not available on Windows")
	}
	root := t.TempDir()
	path := defaultExecLogPath(root)
	want := "USER_DEBUG|[1]|DEBUG|private value\n"
	var stdout bytes.Buffer
	if err := writeDebugLog(path, want, &stdout); err != nil {
		t.Fatal(err)
	}
	assertPathPermissions(t, filepath.Join(root, ".glade"), 0o700)
	assertPathPermissions(t, filepath.Dir(path), 0o700)
	assertPathPermissions(t, path, 0o600)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want || stdout.Len() != 0 {
		t.Fatalf("file = %q, stdout = %q, want log only in file", got, stdout.String())
	}
}

func TestWriteDebugLogPreservesExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not available on Windows")
	}
	directory := filepath.Join(t.TempDir(), "caller-owned")
	if err := os.Mkdir(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "exec.apexlog")
	if err := os.WriteFile(path, []byte("old log with a longer body"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := writeDebugLog(path, "new log\n", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	assertPathPermissions(t, directory, 0o750)
	assertPathPermissions(t, path, 0o640)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new log\n" {
		t.Fatalf("log = %q, want replacement log", got)
	}
}

func TestWriteDebugLogStdout(t *testing.T) {
	var stdout bytes.Buffer
	want := "USER_DEBUG|[1]|DEBUG|stdout value\n"
	if err := writeDebugLog("-", want, &stdout); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func writeDBPermissionsProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	if err := os.Mkdir(filepath.Join(root, "force-app"), 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}
