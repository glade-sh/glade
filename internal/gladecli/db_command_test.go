package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
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

func TestRunDBUIRejectsPublicBindWithoutOptIn(t *testing.T) {
	t.Setenv("GLADE_SERVER_PUBLIC", "")
	root := t.TempDir()
	writeProjectWithWidgetField(t, root, "Name__c")
	var stdout, stderr bytes.Buffer

	code := Run(context.Background(), []string{"db", "ui", "--project", root, "--addr", "0.0.0.0:0", "--no-open"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit = %d, want 1; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "GLADE_SERVER_PUBLIC=1") {
		t.Fatalf("stderr missing public bind guidance: %q", stderr.String())
	}
}

func TestRunDBUIWithOpenURLUsesListenerAddress(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeProjectWithWidgetField(t, root, "Name__c")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var opened string

	err := runDBUIWithOpenURL(ctx, []string{"--project", root, "--addr", "127.0.0.1:0", "--open"}, &bytes.Buffer{}, &bytes.Buffer{}, func(url string) error {
		opened = url
		cancel()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(opened, "http://127.0.0.1:") || !strings.HasSuffix(opened, "/db") {
		t.Fatalf("opened = %q, want exact http://<listener>/db URL", opened)
	}
	addr := strings.TrimSuffix(strings.TrimPrefix(opened, "http://"), "/db")
	if _, _, err := net.SplitHostPort(addr); err != nil {
		t.Fatalf("opened listener address = %q: %v", addr, err)
	}
}

func TestRunDBUIWithOpenURLErrorClosesListener(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeProjectWithWidgetField(t, root, "Name__c")
	wantErr := errors.New("open failed")
	var listener *dbUICloseTrackingListener

	err := runDBUIWithOpenURLAndListen(
		t.Context(),
		[]string{"--project", root, "--addr", "127.0.0.1:0", "--open"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		func(string) error { return wantErr },
		func(network, address string) (net.Listener, error) {
			base, err := net.Listen(network, address)
			if err != nil {
				return nil, err
			}
			if err := base.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second)); err != nil {
				base.Close()
				return nil, err
			}
			listener = &dbUICloseTrackingListener{Listener: base}
			return listener, nil
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if listener.closeCalls != 1 {
		t.Fatalf("listener close calls = %d, want 1", listener.closeCalls)
	}
	if _, err := listener.Listener.Accept(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("underlying listener Accept error = %v, want %v", err, net.ErrClosed)
	}
}

type dbUICloseTrackingListener struct {
	net.Listener
	closeCalls int
}

func (l *dbUICloseTrackingListener) Close() error {
	l.closeCalls++
	return l.Listener.Close()
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

func TestRunDBUIScopesRoutesToDBManager(t *testing.T) {
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
	ready := waitForDBUIReadyFile(t, readyPath)
	client := http.Client{Timeout: 2 * time.Second}

	reset, err := http.NewRequest(http.MethodPost, "http://"+ready.Addr+"/services/data/v60.0/glade/reset", nil)
	if err != nil {
		t.Fatal(err)
	}
	resetRes, err := client.Do(reset)
	if err != nil {
		t.Fatal(err)
	}
	resetRes.Body.Close()
	if resetRes.StatusCode != http.StatusNotFound {
		t.Fatalf("reset status = %d, want 404", resetRes.StatusCode)
	}

	dbRes, err := client.Get("http://" + ready.Addr + "/services/data/v60.0/glade/db-manager/objects")
	if err != nil {
		t.Fatal(err)
	}
	dbRes.Body.Close()
	if dbRes.StatusCode != http.StatusOK {
		t.Fatalf("db-manager status = %d, want 200", dbRes.StatusCode)
	}

	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("db ui did not stop after cancel; stdout=%s stderr=%s", stdout.String(), stderr.String())
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
