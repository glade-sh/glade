package gladecli

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStopTestDaemonPreservesEndpointOnDialError(t *testing.T) {
	root := t.TempDir()
	socket := testdaemon.ServeSocketPath(root)
	pid := testdaemon.ServePIDPath(root)
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Remove(socket)
		_ = os.Remove(pid)
		_ = os.Remove(filepath.Dir(socket))
	})
	// A symlink loop makes dialing fail without establishing that the daemon
	// is absent. Unlike permission failures, this also works when run as root.
	if err := os.Symlink(filepath.Base(socket), socket); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pid, []byte("12345\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := stopTestDaemon(context.Background(), root, &output); err == nil {
		t.Errorf("stop reported success after a dial error: %s", output.String())
	}
	if strings.Contains(output.String(), "test daemon: stopped") {
		t.Error("stop falsely reported stopped")
	}
	for _, path := range []string{socket, pid} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("daemon endpoint removed after dial error: %v", err)
		}
	}
}

func TestStopTestDaemonPreservesUnresponsiveEndpoint(t *testing.T) {
	for _, response := range []string{
		`{"version":2,"op":"pong","ok":true}` + "\n",
		"invalid response\n",
	} {
		t.Run(strings.TrimSpace(response), func(t *testing.T) {
			root := t.TempDir()
			socket := testdaemon.ServeSocketPath(root)
			pid := testdaemon.ServePIDPath(root)
			if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
				t.Fatal(err)
			}
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			t.Cleanup(func() {
				_ = os.Remove(pid)
				_ = os.Remove(filepath.Dir(socket))
			})
			if err := os.WriteFile(pid, []byte("12345\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(time.Second))
				_, _ = testdaemon.DecodeRequestV1(bufio.NewReader(conn))
				_, _ = io.WriteString(conn, response)
			}()
			var output bytes.Buffer
			err = stopTestDaemon(context.Background(), root, &output)
			if err == nil {
				t.Errorf("stop reported success for an unresponsive daemon: %s", output.String())
			}
			if strings.Contains(output.String(), "test daemon: stopped") {
				t.Error("stop falsely reported stopped")
			}
			for _, path := range []string{socket, pid} {
				if _, err := os.Stat(path); err != nil {
					t.Errorf("live daemon endpoint removed: %v", err)
				}
			}
		})
	}
}

func TestTestOneShotCacheStatusExplainsParallelMethodBypass(t *testing.T) {
	restoreDiskCache := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restoreDiskCache)
	got := testOneShotCacheStatus(t.TempDir(), apextest.Options{
		ParallelMethods: true,
		Parallelism:     2,
	})
	for _, want := range []string{
		"bypassed for parallel methods with more than one worker",
		"glade test serve",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status missing %q: %s", want, got)
		}
	}
}

func TestTestOneShotCacheStatusExplainsNoCache(t *testing.T) {
	got := testOneShotCacheStatus(t.TempDir(), apextest.Options{NoDiskCache: true})
	for _, want := range []string{
		"disabled by --no-cache",
		"will not be read or written",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status missing %q: %s", want, got)
		}
	}
}

func TestTestWizardReportsFreshCacheAndOneShotParallelBypass(t *testing.T) {
	restoreDiskCache := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restoreDiskCache)
	previousGOMAXPROCS := runtime.GOMAXPROCS(2)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousGOMAXPROCS) })

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFreshTestStartupCache(t, root)

	var output bytes.Buffer
	if err := writeTestWizard(context.Background(), root, &output); err != nil {
		t.Fatalf("writeTestWizard() error = %v", err)
	}
	for _, want := range []string{
		"cache: fresh\n",
		"one-shot cache: bypassed for parallel methods with more than one worker; the startup cache will not be read or written for this run. Use glade test serve --project " + root,
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("wizard output missing %q:\n%s", want, output.String())
		}
	}
}

func writeFreshTestStartupCache(t *testing.T, root string) {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatalf("project.Load() error = %v", err)
	}
	entry := startupcache.NewEntry(root, p, typesys.Build(p, gladeschema.Schema{}), storage.NewOrgState(), startupcache.CompiledRuntime{})
	if err := startupcache.Write(&entry, startupcache.SubdirTest); err != nil {
		t.Fatalf("startupcache.Write() error = %v", err)
	}
}
