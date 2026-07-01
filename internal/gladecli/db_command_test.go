package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseDBUIOptions(t *testing.T) {
	opts, err := parseDBUIOptions([]string{
		"--project", "/tmp/project",
		"--env", "qa",
		"--addr", "127.0.0.1:4999",
		"--ready-file", "/tmp/ready.json",
		"--no-open",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts.root != "/tmp/project" || opts.envName != "qa" || opts.addr != "127.0.0.1:4999" || opts.readyFile != "/tmp/ready.json" || opts.open {
		t.Fatalf("opts = %#v", opts)
	}
}

func TestParseDBUIOptionsPort(t *testing.T) {
	opts, err := parseDBUIOptions([]string{"--port", "0"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.addr != "127.0.0.1:0" {
		t.Fatalf("addr = %q", opts.addr)
	}
}

func TestRunDBUIReadyFileUsesDefaultProjectEnv(t *testing.T) {
	root := t.TempDir()
	writeProjectWithWidgetField(t, root, "Name__c")
	readyPath := filepath.Join(t.TempDir(), "ready.json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	var stdout, stderr bytes.Buffer
	go func() {
		done <- Run(ctx, []string{"db", "ui", "--project", root, "--addr", "127.0.0.1:0", "--no-open", "--ready-file", readyPath}, &stdout, &stderr)
	}()
	got := waitForDBUIReadyFile(t, readyPath)
	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("db ui did not stop after cancel; stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	wantDB := filepath.Join(root, ".glade", "envs", "dev.sqlite")
	if got.URL == "" || !strings.HasSuffix(got.URL, "/db") || filepath.Clean(got.DB) != filepath.Clean(wantDB) || got.Project != root || got.Env != "dev" {
		t.Fatalf("ready = %#v, want db %s", got, wantDB)
	}
	if !strings.Contains(stdout.String(), "Glade db ui") || !strings.Contains(stdout.String(), "/db") {
		t.Fatalf("stdout missing startup summary: %s", stdout.String())
	}
}

func TestWriteDBUIReadyFile(t *testing.T) {
	readyPath := filepath.Join(t.TempDir(), "ready.json")
	dbPath := filepath.Join(t.TempDir(), "project", ".glade", "envs", "dev.sqlite")
	if err := writeDBUIReadyFile(readyPath, "127.0.0.1:43210", dbPath, "/tmp/project", "dev"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(readyPath)
	if err != nil {
		t.Fatal(err)
	}
	var got dbUIReadyFile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("ready file is not JSON: %v\n%s", err, data)
	}
	if got.URL != "http://127.0.0.1:43210/db" || got.Addr != "127.0.0.1:43210" || got.DB == "" || got.Project != "/tmp/project" || got.Env != "dev" {
		t.Fatalf("ready = %#v", got)
	}
}

func waitForDBUIReadyFile(t *testing.T, path string) dbUIReadyFile {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			var ready dbUIReadyFile
			if err := json.Unmarshal(data, &ready); err != nil {
				t.Fatalf("ready file is not JSON: %v\n%s", err, data)
			}
			return ready
		}
		lastErr = err
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("ready file was not written: %v", lastErr)
	return dbUIReadyFile{}
}
