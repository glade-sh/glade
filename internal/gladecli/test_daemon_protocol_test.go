package gladecli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/trace"
)

func TestTryTestServerRunFailsClosedOnProtocolMismatch(t *testing.T) {
	root := t.TempDir()
	writeServeTestProject(t, root)
	stop := startLegacyProtocolStub(t, testdaemon.ServeSocketPath(root))
	defer stop()

	var stdout bytes.Buffer
	_, used, err := tryTestServerRun(
		context.Background(), root, false, "WarmOneTest", "", "console", "", false, &stdout,
	)
	if err == nil {
		t.Fatal("auto-connect accepted an incompatible responding server")
	}
	if !used {
		t.Fatal("protocol mismatch was reported as an absent server and would silently fall back locally")
	}
	message := strings.ToLower(err.Error())
	for _, want := range []string{"protocol", "restart", "test server"} {
		if !strings.Contains(message, want) {
			t.Fatalf("protocol mismatch error = %q, want %q guidance", err, want)
		}
	}
	if stdout.Len() != 0 {
		t.Fatalf("protocol mismatch wrote test output: %q", stdout.String())
	}
}

func TestConnectedRunKeepsArtifactPathsClientLocal(t *testing.T) {
	root := t.TempDir()
	writeServeTestProject(t, root)
	historyPath := defaultCLIDurationHistoryPath(root)
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(historyPath, []byte(`{
  "classDurations": {"WarmOneTest": 901},
  "methodDurations": {"WarmOneTest.fails": 902}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
		switch request.Op {
		case testdaemon.OpPing:
			return testdaemon.ResponseV1{
				Version: testdaemon.ProtocolVersionV1,
				Op:      testdaemon.OpPong,
				OK:      true,
				Ready:   true,
				Project: root,
			}
		case testdaemon.OpRun:
			return testdaemon.ResponseV1{
				Version: testdaemon.ProtocolVersionV1,
				Op:      testdaemon.OpRunResult,
				ID:      request.ID,
				OK:      true,
				Ready:   true,
				Project: root,
				Run: &testreport.Run{Suites: []testreport.Suite{{
					Name: "WarmOneTest",
					Cases: []testreport.Case{{
						ClassName:  "WarmOneTest",
						MethodName: "fails",
						Status:     testreport.StatusFail,
						DurationMS: 1801,
						Problem:    &testreport.Problem{Type: "Assertion", Message: "expected failure"},
						Trace: []trace.Event{
							trace.Instant("test", "apex", 1, map[string]any{"class": "WarmOneTest"}),
						},
					}},
				}}},
			}
		default:
			return testdaemon.ResponseV1{
				Version: testdaemon.ProtocolVersionV1,
				Op:      testdaemon.OpError,
				ID:      request.ID,
				Error:   "unexpected operation",
			}
		}
	})
	defer stub.Close()

	artifactRoot := t.TempDir()
	tracePath := filepath.Join(artifactRoot, "run.trace.json")
	junitPath := filepath.Join(artifactRoot, "junit.xml")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"test",
		"--project", root,
		"--connect",
		"--json",
		"--no-progress",
		"--trace", tracePath,
		"--junit", junitPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want failing test exit 1\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}

	runRequest := stub.WaitFor(t, testdaemon.OpRun)
	if runRequest.Run == nil {
		t.Fatal("connected run omitted V1 run policy")
	}
	if got, want := runRequest.Run.TimeoutMS, int64(300_000); got != want {
		t.Fatalf("default timeout = %d, want %d", got, want)
	}
	if !runRequest.Run.ParallelMethods || runRequest.Run.Parallelism < 1 {
		t.Fatalf("default parallel policy = methods:%v parallelism:%d", runRequest.Run.ParallelMethods, runRequest.Run.Parallelism)
	}
	if !runRequest.Run.TraceAll {
		t.Fatal("trace collection was not requested from the server")
	}
	if got := runRequest.Run.ClassDurationMS["WarmOneTest"]; got != 901 {
		t.Fatalf("class duration = %d, want 901", got)
	}
	if got := runRequest.Run.MethodDurationMS["WarmOneTest.fails"]; got != 902 {
		t.Fatalf("method duration = %d, want 902", got)
	}
	encoded, err := json.Marshal(runRequest)
	if err != nil {
		t.Fatal(err)
	}
	for _, localPath := range []string{tracePath, junitPath, historyPath} {
		if bytes.Contains(encoded, []byte(localPath)) {
			t.Fatalf("request exposed client-local artifact path %q: %s", localPath, encoded)
		}
	}

	for _, path := range []string{tracePath, junitPath, historyPath} {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			t.Fatalf("client artifact %s: info=%v err=%v", path, info, err)
		}
	}
	traceData, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	var traceDocument trace.Document
	if err := json.Unmarshal(traceData, &traceDocument); err != nil {
		t.Fatalf("decode trace artifact: %v", err)
	}
	if traceDocument.Format != trace.FormatChromeTraceEvent || len(traceDocument.TraceEvents) != 1 {
		t.Fatalf("trace artifact = %#v", traceDocument)
	}
	traceEvent := traceDocument.TraceEvents[0]
	if traceEvent.Name != "test" || traceEvent.Args["class"] != "WarmOneTest" {
		t.Fatalf("trace event = %#v", traceEvent)
	}

	junitData, err := os.ReadFile(junitPath)
	if err != nil {
		t.Fatal(err)
	}
	var junit struct {
		XMLName  xml.Name `xml:"testsuites"`
		Failures int      `xml:"failures,attr"`
		Suites   []struct {
			Cases []struct {
				Name    string `xml:"name,attr"`
				Failure *struct {
					Type    string `xml:"type,attr"`
					Message string `xml:"message,attr"`
				} `xml:"failure"`
			} `xml:"testcase"`
		} `xml:"testsuite"`
	}
	if err := xml.Unmarshal(junitData, &junit); err != nil {
		t.Fatalf("decode JUnit artifact: %v", err)
	}
	if junit.XMLName.Local != "testsuites" || junit.Failures != 1 || len(junit.Suites) != 1 || len(junit.Suites[0].Cases) != 1 {
		t.Fatalf("JUnit artifact = %#v", junit)
	}
	junitCase := junit.Suites[0].Cases[0]
	if junitCase.Name != "fails" || junitCase.Failure == nil || junitCase.Failure.Type != "Assertion" || junitCase.Failure.Message != "expected failure" {
		t.Fatalf("JUnit case = %#v", junitCase)
	}

	history, err := loadCLIDurationHistory(historyPath)
	if err != nil {
		t.Fatalf("decode duration history artifact: %v", err)
	}
	if got, want := history.Classes["WarmOneTest"], mergeCLIDurationMS(901, 1801); got != want {
		t.Fatalf("observed class duration = %d, want merged %d", got, want)
	}
	if got, want := history.Methods["WarmOneTest.fails"], mergeCLIDurationMS(902, 1801); got != want {
		t.Fatalf("observed method duration = %d, want merged %d", got, want)
	}

	failures, err := readLastFailedTests(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(failures, ","), "WarmOneTest.fails"; got != want {
		t.Fatalf("last-failed filters = %q, want %q", got, want)
	}
}

func TestConnectedShardPlanWritesOnlyClientFiles(t *testing.T) {
	root := t.TempDir()
	writeServeTestProject(t, root)
	stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
		switch request.Op {
		case testdaemon.OpPing:
			return testdaemon.ResponseV1{
				Version: testdaemon.ProtocolVersionV1,
				Op:      testdaemon.OpPong,
				OK:      true,
				Ready:   true,
				Project: root,
			}
		case testdaemon.OpRun:
			return testdaemon.ResponseV1{
				Version: testdaemon.ProtocolVersionV1,
				Op:      testdaemon.OpRunResult,
				ID:      request.ID,
				OK:      true,
				Ready:   true,
				Project: root,
				ShardPlan: &testdaemon.ClassShardPlanV1{Shards: []testdaemon.ClassShardV1{
					{Index: 0, TotalDurationMS: 20, Classes: []string{"WarmTwoTest"}},
					{Index: 1, TotalDurationMS: 10, Classes: []string{"WarmOneTest"}},
				}},
			}
		default:
			return testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpError, Error: "unexpected operation"}
		}
	})
	defer stub.Close()

	shardDir := filepath.Join(t.TempDir(), "shards")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"test",
		"--project", root,
		"--connect",
		"--write-class-shards", shardDir,
		"--shard-count", "2",
		"--no-progress",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}

	runRequest := stub.WaitFor(t, testdaemon.OpRun)
	if runRequest.Run == nil || !runRequest.Run.ReturnClassShards || runRequest.Run.ShardCount != 2 {
		t.Fatalf("shard-plan request = %#v", runRequest)
	}
	encoded, err := json.Marshal(runRequest)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(shardDir)) {
		t.Fatalf("request exposed client shard directory: %s", encoded)
	}
	for path, want := range map[string]string{
		filepath.Join(shardDir, "shard-000.txt"): "WarmTwoTest\n",
		filepath.Join(shardDir, "shard-001.txt"): "WarmOneTest\n",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(data) != want {
			t.Fatalf("%s = %q, want %q", path, data, want)
		}
	}
}

func TestConnectedShardPlanPreservesExactSelectorFailures(t *testing.T) {
	root := t.TempDir()
	writeServeTestProject(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, err := testdaemon.NewServer(testdaemon.ServerConfig{
		Root:   root,
		Socket: testdaemon.ServeSocketPath(root),
		Warm:   false,
		Watch:  false,
	})
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe(ctx, io.Discard) }()
	waitForServer(t, ctx, testdaemon.ServeSocketPath(root))

	tests := []struct {
		name        string
		selector    []string
		wantMessage string
	}{
		{
			name:        "missing class",
			selector:    []string{"--class", "MissingTest"},
			wantMessage: `no test class matched --class "MissingTest"`,
		},
		{
			name:        "missing method",
			selector:    []string{"--class", "WarmOneTest", "--method", "missingMethod"},
			wantMessage: `no test method matched --class "WarmOneTest" --method "missingMethod"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shardDir := filepath.Join(t.TempDir(), "shards")
			args := []string{
				"test", "--project", root, "--connect", "--json", "--no-progress",
				"--write-class-shards", shardDir, "--shard-count", "2",
			}
			args = append(args, test.selector...)
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), args, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("exit = %d, want structured selector failure\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q", stderr.String())
			}
			envelope := decodeSelectorFailureEnvelope(t, stdout.Bytes())
			if envelope.Summary.Total != 1 || envelope.Summary.Errors != 1 {
				t.Fatalf("selector summary = %#v", envelope.Summary)
			}
			if got := firstSelectorFailureMessage(envelope.Data); got != test.wantMessage {
				t.Fatalf("selector message = %q, want %q", got, test.wantMessage)
			}
			if _, err := os.Stat(shardDir); !os.IsNotExist(err) {
				t.Fatalf("selector failure wrote shard directory: %v", err)
			}
		})
	}
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve exit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestWriteClassShardsValidatesImplicitParallelismBeforeArtifacts(t *testing.T) {
	root := t.TempDir()
	writeServeTestProject(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, err := testdaemon.NewServer(testdaemon.ServerConfig{
		Root:   root,
		Socket: testdaemon.ServeSocketPath(root),
		Warm:   false,
		Watch:  false,
	})
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe(ctx, io.Discard) }()
	waitForServer(t, ctx, testdaemon.ServeSocketPath(root))

	for _, mode := range []struct {
		name string
		args []string
	}{
		{name: "local", args: []string{"--no-serve"}},
		{name: "connected", args: []string{"--connect"}},
	} {
		for _, count := range []struct {
			value int
			valid bool
		}{
			{value: 1, valid: true},
			{value: testdaemon.MaxClassShardCountV1, valid: true},
			{value: testdaemon.MaxClassShardCountV1 + 1, valid: false},
		} {
			t.Run(mode.name+"/"+strconv.Itoa(count.value), func(t *testing.T) {
				shardDir := filepath.Join(t.TempDir(), "shards")
				args := []string{
					"test", "--project", root, "--no-progress",
					"--write-class-shards", shardDir,
					"--parallelism", strconv.Itoa(count.value),
				}
				args = append(args, mode.args...)
				var stdout, stderr bytes.Buffer
				code := Run(context.Background(), args, &stdout, &stderr)
				if count.valid {
					if code != 0 {
						t.Fatalf("valid implicit shard count exited %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
					}
					entries, err := os.ReadDir(shardDir)
					if err != nil {
						t.Fatalf("read valid shard directory: %v", err)
					}
					if len(entries) != count.value {
						t.Fatalf("shard files = %d, want %d", len(entries), count.value)
					}
					return
				}
				if code == 0 {
					t.Fatalf("oversized implicit shard count succeeded\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
				}
				output := strings.ToLower(stdout.String() + stderr.String())
				if !strings.Contains(output, "shard") || !strings.Contains(output, strconv.Itoa(testdaemon.MaxClassShardCountV1)) {
					t.Fatalf("oversized implicit shard error = %q, want shard maximum guidance", output)
				}
				if _, err := os.Stat(shardDir); !os.IsNotExist(err) {
					t.Fatalf("oversized implicit shard count created artifact directory: %v", err)
				}
			})
		}
	}
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve exit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestConnectedMalformedFailedShardResponseWritesNoArtifacts(t *testing.T) {
	root := t.TempDir()
	writeServeTestProject(t, root)
	stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
		switch request.Op {
		case testdaemon.OpPing:
			return testdaemon.ResponseV1{
				Version: testdaemon.ProtocolVersionV1,
				Op:      testdaemon.OpPong,
				OK:      true,
				Ready:   true,
				Project: root,
			}
		case testdaemon.OpRun:
			return testdaemon.ResponseV1{
				Version: testdaemon.ProtocolVersionV1,
				Op:      testdaemon.OpRunResult,
				ID:      request.ID,
				OK:      false,
				Error:   "deliberate malformed failed run result",
				ShardPlan: &testdaemon.ClassShardPlanV1{Shards: []testdaemon.ClassShardV1{
					{Index: 0, Classes: []string{"WarmOneTest"}},
				}},
			}
		default:
			return testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpError, Error: "unexpected operation"}
		}
	})
	defer stub.Close()

	shardDir := filepath.Join(t.TempDir(), "shards")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"test", "--project", root, "--connect", "--no-progress",
		"--write-class-shards", shardDir, "--shard-count", "1",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("malformed failed run result succeeded\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if output := stdout.String() + stderr.String(); !strings.Contains(output, "deliberate malformed failed run result") {
		t.Fatalf("malformed failed response error = %q", output)
	}
	if _, err := os.Stat(shardDir); !os.IsNotExist(err) {
		t.Fatalf("malformed failed response wrote shard artifacts before validation: %v", err)
	}
}

func TestConnectedShardPlanRequiresExactUniqueIndexesBeforeArtifacts(t *testing.T) {
	tests := []struct {
		name   string
		shards []testdaemon.ClassShardV1
	}{
		{
			name: "too many",
			shards: []testdaemon.ClassShardV1{
				{Index: 0}, {Index: 1}, {Index: 2},
			},
		},
		{
			name: "too few",
			shards: []testdaemon.ClassShardV1{
				{Index: 0},
			},
		},
		{
			name: "duplicate",
			shards: []testdaemon.ClassShardV1{
				{Index: 0}, {Index: 0},
			},
		},
		{
			name: "negative",
			shards: []testdaemon.ClassShardV1{
				{Index: -1}, {Index: 1},
			},
		},
		{
			name: "out of range gap",
			shards: []testdaemon.ClassShardV1{
				{Index: 0}, {Index: 2},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeServeTestProject(t, root)
			stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
				switch request.Op {
				case testdaemon.OpPing:
					return testdaemon.ResponseV1{
						Version: testdaemon.ProtocolVersionV1,
						Op:      testdaemon.OpPong,
						OK:      true,
						Ready:   true,
						Project: root,
					}
				case testdaemon.OpRun:
					return testdaemon.ResponseV1{
						Version:   testdaemon.ProtocolVersionV1,
						Op:        testdaemon.OpRunResult,
						ID:        request.ID,
						OK:        true,
						ShardPlan: &testdaemon.ClassShardPlanV1{Shards: test.shards},
					}
				default:
					return testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpError, Error: "unexpected operation"}
				}
			})
			defer stub.Close()

			shardDir := filepath.Join(t.TempDir(), "shards")
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{
				"test", "--project", root, "--connect", "--no-progress",
				"--write-class-shards", shardDir, "--parallelism", "2",
			}, &stdout, &stderr)
			request := stub.WaitFor(t, testdaemon.OpRun)
			if request.Run == nil || request.Run.ShardCount != 2 || !request.Run.ReturnClassShards {
				t.Fatalf("effective shard request = %#v", request)
			}
			if code == 0 {
				t.Fatalf("malformed shard plan succeeded\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
			}
			if output := strings.ToLower(stdout.String() + stderr.String()); !strings.Contains(output, "shard") || !strings.Contains(output, "plan") {
				t.Fatalf("malformed shard plan error = %q", output)
			}
			if _, err := os.Stat(shardDir); !os.IsNotExist(err) {
				t.Fatalf("malformed shard plan wrote artifacts before validation: %v", err)
			}
		})
	}
}

func TestWriteClassShardsDefensivelyRejectsInvalidCountsBeforeArtifacts(t *testing.T) {
	for _, count := range []int{-1, 0, testdaemon.MaxClassShardCountV1 + 1} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			shardDir := filepath.Join(t.TempDir(), "shards")
			err := writeCLIClassShards(shardDir, nil, "", count)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "shard") {
				t.Fatalf("write shard count %d: err=%v, want bounded shard error", count, err)
			}
			if _, statErr := os.Stat(shardDir); !os.IsNotExist(statErr) {
				t.Fatalf("invalid shard count %d created artifact directory: %v", count, statErr)
			}
		})
	}
}

type protocolV1Stub struct {
	listener net.Listener
	requests chan testdaemon.RequestV1
	errors   chan error
	respond  func(testdaemon.RequestV1) testdaemon.ResponseV1
}

func startProtocolV1Stub(
	t *testing.T,
	root string,
	respond func(testdaemon.RequestV1) testdaemon.ResponseV1,
) *protocolV1Stub {
	t.Helper()
	socket := testdaemon.ServeSocketPath(root)
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	stub := &protocolV1Stub{
		listener: listener,
		requests: make(chan testdaemon.RequestV1, 8),
		errors:   make(chan error, 8),
		respond:  respond,
	}
	go stub.Serve()
	return stub
}

func (s *protocolV1Stub) Serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				s.errors <- err
			}
			return
		}
		func() {
			defer conn.Close()
			request, err := testdaemon.DecodeRequestV1(bufio.NewReader(conn))
			if err != nil {
				s.errors <- err
				return
			}
			s.requests <- request
			if err := testdaemon.EncodeResponseV1(conn, s.respond(request)); err != nil {
				s.errors <- err
			}
		}()
	}
}

func (s *protocolV1Stub) WaitFor(t *testing.T, operation string) testdaemon.RequestV1 {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case err := <-s.errors:
			t.Fatalf("protocol stub: %v", err)
		case request := <-s.requests:
			if request.Op == operation {
				return request
			}
		case <-timer.C:
			t.Fatalf("protocol stub did not receive %q", operation)
		}
	}
}

func (s *protocolV1Stub) Close() {
	_ = s.listener.Close()
}

func startLegacyProtocolStub(t *testing.T, socket string) func() {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				var request testdaemon.Request
				if err := json.NewDecoder(conn).Decode(&request); err != nil {
					return
				}
				response := testdaemon.Response{Op: testdaemon.OpPong, OK: true, Ready: true}
				if request.Op == testdaemon.OpRun {
					response = testdaemon.Response{
						Op: testdaemon.OpRunResult,
						OK: true,
						Run: &testreport.Run{Suites: []testreport.Suite{{
							Name:  "WarmOneTest",
							Cases: []testreport.Case{{ClassName: "WarmOneTest", MethodName: "passes", Status: testreport.StatusPass}},
						}}},
					}
				}
				_ = json.NewEncoder(conn).Encode(response)
			}()
		}
	}()
	return func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
}
