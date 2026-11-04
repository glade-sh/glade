package gladecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateRejectsFailedInstallerDownload(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "empty", body: ""},
		{name: "partial", body: "exit 0\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", dir)
			t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "xdg"))
			t.Setenv("GLADE_HOME", "")
			t.Setenv("GLADE_ROOT", "")
			curl := filepath.Join(dir, "curl")
			script := "#!/bin/sh\n" + "printf '%s' " + shellQuote(tc.body) + "\nexit 22\n"
			if err := os.WriteFile(curl, []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("GLADE_UPDATE_ALLOW_SHELL", "1")
			var stdout, stderr bytes.Buffer
			if err := runUpdate(context.Background(), nil, &stdout, &stderr); err == nil {
				t.Fatalf("failed installer download returned success; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestUpdateDoesNotExecuteDownloadedInstaller(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	marker := filepath.Join(dir, "executed")
	payload := "printf compromised > " + shellQuote(marker) + "\n"
	curl := filepath.Join(dir, "curl")
	if err := os.WriteFile(curl, []byte("#!/bin/sh\nprintf '%s' "+shellQuote(payload)+"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GLADE_UPDATE_ALLOW_SHELL", "1")
	var stdout, stderr bytes.Buffer
	_ = runUpdate(context.Background(), nil, &stdout, &stderr)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("downloaded installer bytes were executed")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
