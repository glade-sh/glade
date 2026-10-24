package gladecli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testdaemon"
)

func TestStopTestDaemonWaitsForActiveRunCleanup(t *testing.T) {
	for _, route := range []string{"direct", "cli"} {
		t.Run(route, func(t *testing.T) {
			daemon := startDrainingStopDaemon(t)
			stopped := make(chan daemonStopResult, 1)
			go func() { stopped <- invokeDaemonStop(daemon.root, route) }()
			awaitDaemonStopSignal(t, daemon.shuttingDown, "shutdown request")
			assertDaemonStopWaits(t, stopped, daemon.release)
			if _, err := os.Stat(testdaemon.ServePIDPath(daemon.root)); !os.IsNotExist(err) {
				t.Errorf("PID file remains after successful stop: %v", err)
			}
		})
	}
}

func TestStopTestDaemonReportsActiveRunCleanupTimeout(t *testing.T) {
	daemon := startDrainingStopDaemon(t)
	result := invokeDaemonStop(daemon.root, "direct")
	if result.err == nil || !strings.Contains(result.err.Error(), "did not stop") {
		t.Errorf("stop did not report blocked run cleanup: err=%v stdout=%q", result.err, result.output)
	}
	if strings.Contains(result.output, "test daemon: stopped") {
		t.Error("stop falsely reported stopped while run cleanup was blocked")
	}
	if _, err := os.Stat(testdaemon.ServePIDPath(daemon.root)); err != nil {
		t.Errorf("stop removed the active daemon PID: %v", err)
	}
}

func TestRunTestDaemonStopPreservesLivePIDWithoutResponsiveSocket(t *testing.T) {
	for _, endpoint := range []string{"missing", "refused"} {
		t.Run(endpoint, func(t *testing.T) {
			root := t.TempDir()
			socket, pid := prepareStopEndpoint(t, root)
			if endpoint == "refused" {
				listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
				if err != nil {
					t.Fatal(err)
				}
				listener.SetUnlinkOnClose(false)
				if err := listener.Close(); err != nil {
					t.Fatal(err)
				}
			}
			result := invokeDaemonStop(root, "cli")
			if result.err == nil || strings.Contains(result.output, "test daemon: stopped") {
				t.Errorf("stop reported success while the recorded PID was alive: err=%v stdout=%q", result.err, result.output)
			}
			if _, err := os.Stat(pid); err != nil {
				t.Errorf("stop removed the live PID after a %s dial: %v", endpoint, err)
			}
			if endpoint == "refused" {
				if _, err := os.Stat(socket); err != nil {
					t.Errorf("stop removed the refused endpoint with a live PID: %v", err)
				}
			}
		})
	}
}

func TestRunTestDaemonStopPreservesUnresponsiveEndpoint(t *testing.T) {
	for _, response := range []string{"old-protocol", "silent"} {
		t.Run(response, func(t *testing.T) {
			root := t.TempDir()
			socket, pid := prepareStopEndpoint(t, root)
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				_, _ = testdaemon.DecodeRequestV1(bufio.NewReader(conn))
				if response == "old-protocol" {
					_, _ = io.WriteString(conn, `{"version":2,"op":"pong","ok":true}`+"\n")
				} else {
					<-ctx.Done()
				}
			}()
			var stdout, stderr bytes.Buffer
			code := Run(ctx, []string{"test", "daemon", "stop", "--project", root}, &stdout, &stderr)
			cancel()
			awaitDaemonStopSignal(t, done, "unresponsive endpoint exit")
			if code == 0 || strings.Contains(stdout.String(), "test daemon: stopped") {
				t.Fatalf("unresponsive stop: exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			for _, path := range []string{socket, pid} {
				if _, err := os.Stat(path); err != nil {
					t.Errorf("stop removed an unresponsive endpoint: %v", err)
				}
			}
		})
	}
}

func TestRunTestDaemonStopWaitsForProjectRequestDrain(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			t.Cleanup(apextest.InvalidateRuntimeCaches)
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			classes := filepath.Join(root, "force-app/main/default/classes")
			writeTestFile(t, filepath.Join(classes, "StoppableTest.cls"), `@IsTest private class StoppableTest { @IsTest static void passes() { System.assertEquals(2, 1 + 1); } }`)
			writeTestFile(t, filepath.Join(classes, "StoppableTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			server, err := testdaemon.NewServer(testdaemon.ServerConfig{Root: root})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			served := make(chan error, 1)
			go func() { served <- server.ListenAndServe(ctx, io.Discard) }()
			t.Cleanup(func() {
				cancel()
				if err := server.Close(); err != nil {
					t.Error(err)
				}
				select {
				case err := <-served:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(3 * time.Second):
					t.Error("project server did not exit")
				}
			})
			socket := testdaemon.ServeSocketPath(root)
			waitForServer(t, ctx, socket)
			request := testdaemon.RunRequestV1{Filter: "StoppableTest", Parallelism: 1}
			run, err := testdaemon.RunV1(ctx, socket, request)
			if err != nil || run.Run == nil || run.Run.Summary().Passed != 1 {
				t.Fatalf("project run: response=%#v err=%v", run, err)
			}

			// Hold a second run frame in transit so shutdown must drain a real
			// admitted connection before removing the server's PID file.
			pending, err := net.Dial("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			defer pending.Close()
			var frame bytes.Buffer
			if err := testdaemon.EncodeRequestV1(&frame, testdaemon.RequestV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpRun, Run: &request}); err != nil {
				t.Fatal(err)
			}
			if _, err := pending.Write(bytes.TrimSuffix(frame.Bytes(), []byte{'\n'})); err != nil {
				t.Fatal(err)
			}
			if _, err := testdaemon.Ping(ctx, socket); err != nil {
				t.Fatal(err)
			}
			stopped := make(chan daemonStopResult, 1)
			go func() { stopped <- invokeDaemonStop(root, "cli") }()
			deadline := time.Now().Add(time.Second)
			for fileExists(socket) && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			if _, err := os.Stat(socket); !os.IsNotExist(err) {
				t.Fatalf("shutdown did not close its listener: %v", err)
			}
			assertDaemonStopWaits(t, stopped, func() { _ = pending.Close() })
		})
	}
}

type daemonStopResult struct {
	err    error
	output string
}

func invokeDaemonStop(root, route string) daemonStopResult {
	var stdout, stderr bytes.Buffer
	var err error
	if route == "direct" {
		err = stopTestDaemon(context.Background(), root, &stdout)
	} else if code := Run(context.Background(), []string{"test", "daemon", "stop", "--project", root}, &stdout, &stderr); code != 0 {
		err = fmt.Errorf("exit %d: %s", code, stderr.String())
	}
	return daemonStopResult{err: err, output: stdout.String()}
}

func assertDaemonStopWaits(t *testing.T, stopped <-chan daemonStopResult, release func()) {
	t.Helper()
	var result daemonStopResult
	early := false
	select {
	case result = <-stopped:
		early = true
		t.Errorf("stop returned before active request cleanup: err=%v stdout=%q", result.err, result.output)
	case <-time.After(100 * time.Millisecond):
	}
	release()
	if !early {
		select {
		case result = <-stopped:
		case <-time.After(3 * time.Second):
			t.Fatal("stop did not complete after request cleanup")
		}
	}
	if result.err != nil || !strings.Contains(result.output, "test daemon: stopped") {
		t.Errorf("stop after cleanup: err=%v stdout=%q", result.err, result.output)
	}
}

func prepareStopEndpoint(t *testing.T, root string) (string, string) {
	t.Helper()
	socket, pid := testdaemon.ServeSocketPath(root), testdaemon.ServePIDPath(root)
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pid, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Remove(socket)
		_ = os.Remove(pid)
		_ = os.Remove(filepath.Dir(socket))
	})
	return socket, pid
}

type drainingStopDaemon struct {
	root         string
	shuttingDown chan struct{}
	release      func()
}

func startDrainingStopDaemon(t *testing.T) drainingStopDaemon {
	t.Helper()
	root := t.TempDir()
	socket, pid := prepareStopEndpoint(t, root)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	released, started, shutdown, done := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(released) }) }
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(func() {
		release()
		cancel()
		_ = listener.Close()
		awaitDaemonStopSignal(t, done, "draining daemon exit")
	})
	go func() {
		defer close(done)
		var handlers sync.WaitGroup
		defer handlers.Wait()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			handlers.Go(func() {
				defer conn.Close()
				request, err := testdaemon.DecodeRequestV1(bufio.NewReader(conn))
				if err != nil {
					return
				}
				response := testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, OK: true}
				switch request.Op {
				case testdaemon.OpPing:
					response.Op = testdaemon.OpPong
				case testdaemon.OpRun:
					close(started)
					<-released
					_ = os.Remove(pid)
					response.Op, response.OK, response.Error = testdaemon.OpError, false, "run canceled by shutdown"
				case testdaemon.OpShutdown:
					response.Op = testdaemon.OpShutdownAck
					_ = testdaemon.EncodeResponseV1(conn, response)
					_ = listener.Close()
					close(shutdown)
					return
				}
				_ = testdaemon.EncodeResponseV1(conn, response)
			})
		}
	}()
	go func() { _, _ = testdaemon.RunV1(ctx, socket, testdaemon.RunRequestV1{Parallelism: 1}) }()
	awaitDaemonStopSignal(t, started, "active run")
	return drainingStopDaemon{root: root, shuttingDown: shutdown, release: release}
}

func awaitDaemonStopSignal(t *testing.T, done <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}
