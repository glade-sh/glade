package testdaemon

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
	"github.com/glade-sh/glade/internal/watch"
)

func TestProtocolWiringV1MapsEveryRunOptionAndLimitCap(t *testing.T) {
	wantOptions := apextest.Options{
		Filter:               "Invoice",
		SelectedClasses:      []string{"InvoiceTest", "TaxTest"},
		SelectedMethod:       "calculatesTotal",
		LimitMode:            vm.LimitModeStrict,
		LimitCaps:            distinctProtocolWiringLimitCaps(),
		LimitCapsSet:         true,
		TraceBlocked:         true,
		TraceAll:             true,
		SlowTestThresholdMS:  211,
		TimeoutMS:            300_017,
		Parallelism:          7,
		ParallelMethods:      false,
		NoDiskCache:          true,
		ClassDurationMS:      map[string]int64{"InvoiceTest": 1201, "TaxTest": 1202},
		MethodDurationMS:     map[string]int64{"InvoiceTest.calculatesTotal": 1301},
		PerfCounters:         true,
		PreRunPhaseDurations: apextest.PreRunPhaseDurations{},
	}
	wantRequest := RunRequestV1{
		Filter:              "Invoice",
		ChangedSince:        "origin/main",
		SelectedClasses:     []string{"InvoiceTest", "TaxTest"},
		SelectedMethod:      "calculatesTotal",
		LimitMode:           "strict",
		LimitCaps:           distinctProtocolWiringLimitCapsV1(),
		LimitCapsSet:        true,
		TraceBlocked:        true,
		TraceAll:            true,
		SlowTestThresholdMS: 211,
		TimeoutMS:           300_017,
		Parallelism:         7,
		ParallelMethods:     false,
		NoDiskCache:         true,
		ClassDurationMS:     map[string]int64{"InvoiceTest": 1201, "TaxTest": 1202},
		MethodDurationMS:    map[string]int64{"InvoiceTest.calculatesTotal": 1301},
		PerfCounters:        true,
		ShardCount:          5,
		ShardIndex:          3,
		ReturnClassShards:   true,
	}

	gotRequest := runRequestV1FromOptions(wantOptions, "origin/main", 5, 3, true)
	if !reflect.DeepEqual(gotRequest, wantRequest) {
		t.Fatalf("client option mapping mismatch:\n got: %#v\nwant: %#v", gotRequest, wantRequest)
	}

	gotOptions := apexOptionsFromRunRequestV1(gotRequest)
	if !reflect.DeepEqual(gotOptions, wantOptions) {
		t.Fatalf("server option mapping mismatch:\n got: %#v\nwant: %#v", gotOptions, wantOptions)
	}

	if got, want := reflect.TypeOf(gotRequest.LimitCaps).NumField(), 19; got != want {
		t.Fatalf("LimitCapsV1 field count = %d, want %d", got, want)
	}
}

func TestProtocolWiringV1CopiesCallerOwnedSlicesAndMaps(t *testing.T) {
	options := apextest.Options{
		SelectedClasses:  []string{"InvoiceTest"},
		ClassDurationMS:  map[string]int64{"InvoiceTest": 101},
		MethodDurationMS: map[string]int64{"InvoiceTest.passes": 102},
	}
	request := runRequestV1FromOptions(options, "", 0, 0, false)

	options.SelectedClasses[0] = "mutated"
	options.ClassDurationMS["InvoiceTest"] = 999
	options.MethodDurationMS["InvoiceTest.passes"] = 999
	if got := request.SelectedClasses; !reflect.DeepEqual(got, []string{"InvoiceTest"}) {
		t.Fatalf("request retained caller selected-classes slice: %#v", got)
	}
	if got := request.ClassDurationMS["InvoiceTest"]; got != 101 {
		t.Fatalf("request retained caller class-duration map: %d", got)
	}
	if got := request.MethodDurationMS["InvoiceTest.passes"]; got != 102 {
		t.Fatalf("request retained caller method-duration map: %d", got)
	}

	mapped := apexOptionsFromRunRequestV1(request)
	request.SelectedClasses[0] = "mutated again"
	request.ClassDurationMS["InvoiceTest"] = 888
	request.MethodDurationMS["InvoiceTest.passes"] = 888
	if got := mapped.SelectedClasses; !reflect.DeepEqual(got, []string{"InvoiceTest"}) {
		t.Fatalf("runner options retained request selected-classes slice: %#v", got)
	}
	if got := mapped.ClassDurationMS["InvoiceTest"]; got != 101 {
		t.Fatalf("runner options retained request class-duration map: %d", got)
	}
	if got := mapped.MethodDurationMS["InvoiceTest.passes"]; got != 102 {
		t.Fatalf("runner options retained request method-duration map: %d", got)
	}
}

func TestProtocolWiringV1RejectsUnsafeShardCountsBeforePlanning(t *testing.T) {
	tests := []struct {
		name       string
		count      int
		returnPlan bool
		wantError  bool
	}{
		{name: "ordinary unsharded run", count: 0},
		{name: "plan requires positive count", count: 0, returnPlan: true, wantError: true},
		{name: "negative", count: -1, returnPlan: true, wantError: true},
		{name: "maximum supported", count: 1024, returnPlan: true},
		{name: "over supported maximum", count: 1025, returnPlan: true, wantError: true},
		{name: "machine maximum", count: math.MaxInt, returnPlan: true, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRunRequestV1(RunRequestV1{
				Parallelism:       1,
				ShardCount:        test.count,
				ReturnClassShards: test.returnPlan,
			})
			if test.wantError {
				if err == nil || !strings.Contains(strings.ToLower(err.Error()), "shard") {
					t.Fatalf("validate shard count %d, returnPlan=%v: err=%v, want bounded shard error", test.count, test.returnPlan, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validate shard count %d, returnPlan=%v: %v", test.count, test.returnPlan, err)
			}
		})
	}

	server := protocolWiringReadyServer(t)
	rejected := exchangeProtocolWiringV1(t, server, RequestV1{
		Version: ProtocolVersionV1,
		Op:      OpRun,
		Run: &RunRequestV1{
			Parallelism:       1,
			ShardCount:        1025,
			ReturnClassShards: true,
		},
	})
	if rejected.Op != OpError || rejected.OK || !strings.Contains(strings.ToLower(rejected.Error), "shard") {
		t.Fatalf("oversized shard response = %#v", rejected)
	}
	ping := exchangeProtocolWiringV1(t, server, RequestV1{Version: ProtocolVersionV1, Op: OpPing})
	if ping.Op != OpPong || !ping.OK {
		t.Fatalf("ping after rejected shard request = %#v", ping)
	}
}

func TestProtocolWiringV1ClientBackgroundRequestHasBoundedIO(t *testing.T) {
	previousTimeout := clientControlIOTimeoutV1
	clientControlIOTimeoutV1 = 100 * time.Millisecond
	defer func() { clientControlIOTimeoutV1 = previousTimeout }()
	socket, listener := protocolWiringUnixListener(t)
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	result := make(chan error, 1)
	go func() {
		_, err := PingV1(context.Background(), socket)
		result <- err
	}()
	conn := <-accepted
	defer conn.Close()
	if _, err := DecodeRequestV1(bufio.NewReader(conn)); err != nil {
		t.Fatalf("decode client ping: %v", err)
	}

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("nonresponding server returned nil error")
		}
	case <-time.After(time.Second):
		t.Fatal("Background client request remained blocked beyond the bounded default")
	}
}

func TestProtocolWiringV1ClientRunHasSeparateBoundedDefault(t *testing.T) {
	previousTimeout := clientRunIOTimeoutV1
	clientRunIOTimeoutV1 = 100 * time.Millisecond
	defer func() { clientRunIOTimeoutV1 = previousTimeout }()
	socket, listener := protocolWiringUnixListener(t)
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	result := make(chan error, 1)
	go func() {
		_, err := RunV1(context.Background(), socket, RunRequestV1{Parallelism: 1})
		result <- err
	}()
	conn := <-accepted
	defer conn.Close()
	if _, err := DecodeRequestV1(bufio.NewReader(conn)); err != nil {
		t.Fatalf("decode client run: %v", err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("nonresponding run server returned nil error")
		}
	case <-time.After(time.Second):
		t.Fatal("Background run request ignored its separate bounded default")
	}
}

func TestProtocolWiringV1CallerDeadlineOverridesClientDefault(t *testing.T) {
	previousTimeout := clientControlIOTimeoutV1
	clientControlIOTimeoutV1 = 5 * time.Second
	defer func() { clientControlIOTimeoutV1 = previousTimeout }()
	socket, listener := protocolWiringUnixListener(t)
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := PingV1(ctx, socket)
		result <- err
	}()
	conn := <-accepted
	defer conn.Close()
	if _, err := DecodeRequestV1(bufio.NewReader(conn)); err != nil {
		t.Fatalf("decode client ping: %v", err)
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("caller-deadline request error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("caller deadline did not override the longer client default")
	}
}

func TestProtocolWiringV1ClientCancellationInterruptsIOAfterDial(t *testing.T) {
	socket, listener := protocolWiringUnixListener(t)
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := PingV1(ctx, socket)
		result <- err
	}()
	conn := <-accepted
	defer conn.Close()
	if _, err := DecodeRequestV1(bufio.NewReader(conn)); err != nil {
		t.Fatalf("decode client ping: %v", err)
	}
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled request error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("caller cancellation did not interrupt connected client I/O")
	}
}

func TestProtocolWiringV1DisconnectCancelsRunAndBoundsAdmission(t *testing.T) {
	server := protocolWiringReadyServer(t)
	started := make(chan string, 2)
	canceled := make(chan error, 1)
	var calls atomic.Int32
	server.runRequestV1 = func(ctx context.Context, request RunRequestV1) (testreport.Run, watch.TestSelection, *ClassShardPlanV1, error) {
		calls.Add(1)
		started <- request.Filter
		<-ctx.Done()
		canceled <- ctx.Err()
		return testreport.Run{}, watch.TestSelection{}, nil, ctx.Err()
	}

	active := startProtocolWiringExchange(t, server, "active")
	select {
	case got := <-started:
		if got != "active" {
			t.Fatalf("first admitted run = %q, want active", got)
		}
	case <-time.After(time.Second):
		t.Fatal("active run did not start")
	}

	// Admission holds one active and one queued request. Of two additional
	// requests, exactly one must receive the stable busy response immediately.
	contenders := []*protocolWiringExchange{
		startProtocolWiringExchange(t, server, "contender-1"),
		startProtocolWiringExchange(t, server, "contender-2"),
	}
	busyIndex := -1
	select {
	case result := <-contenders[0].result:
		assertProtocolWiringBusyResponse(t, result)
		busyIndex = 0
	case result := <-contenders[1].result:
		assertProtocolWiringBusyResponse(t, result)
		busyIndex = 1
	case <-time.After(time.Second):
		t.Fatal("third admitted run was queued instead of receiving the bounded busy response")
	}
	queued := contenders[1-busyIndex]
	select {
	case result := <-queued.result:
		t.Fatalf("allowed queued run completed while active was held: response=%#v err=%v", result.response, result.err)
	default:
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("run calls while active = %d, want exactly one", got)
	}

	// Disconnecting the waiter must remove it from admission without running it.
	_ = queued.client.Close()
	select {
	case <-queued.done:
	case <-time.After(time.Second):
		t.Fatal("disconnected queued run remained in admission")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("disconnected queued request entered run: calls=%d", got)
	}

	// Disconnecting the active client must cancel the context seen by the run.
	_ = active.client.Close()
	select {
	case err := <-canceled:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("active run cancellation = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("active run did not observe client disconnect cancellation")
	}
	select {
	case <-active.done:
	case <-time.After(time.Second):
		t.Fatal("active connection did not release after cancellation")
	}
}

func TestProtocolWiringV1CompletedRunStopsDisconnectMonitor(t *testing.T) {
	server := protocolWiringReadyServer(t)
	server.runRequestV1 = func(context.Context, RunRequestV1) (testreport.Run, watch.TestSelection, *ClassShardPlanV1, error) {
		return testreport.Run{Name: "completed"}, watch.TestSelection{}, nil, nil
	}

	exchange := startProtocolWiringExchange(t, server, "completed")
	select {
	case result := <-exchange.result:
		if result.err != nil {
			t.Fatalf("decode completed run: %v", result.err)
		}
		if result.response.Op != OpRunResult || !result.response.OK || result.response.Run == nil || result.response.Run.Name != "completed" {
			t.Fatalf("completed response = %#v", result.response)
		}
	case <-time.After(time.Second):
		t.Fatal("normal run did not return its response")
	}
	select {
	case <-exchange.done:
	case <-time.After(time.Second):
		t.Fatal("disconnect monitor outlived a normally completed connection")
	}
}

func TestProtocolWiringV1CanceledContextNeverEntersRun(t *testing.T) {
	server := protocolWiringReadyServer(t)
	var calls atomic.Int32
	server.runRequestV1 = func(context.Context, RunRequestV1) (testreport.Run, watch.TestSelection, *ClassShardPlanV1, error) {
		calls.Add(1)
		return testreport.Run{}, watch.TestSelection{}, nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response := server.handleV1(ctx, RequestV1{
		Version: ProtocolVersionV1,
		Op:      OpRun,
		Run:     &RunRequestV1{Parallelism: 1},
	})
	if response.Op != OpError || response.OK || !strings.Contains(response.Error, context.Canceled.Error()) {
		t.Fatalf("canceled run response = %#v", response)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("already-canceled context entered run hook %d times", got)
	}
	if len(server.runAdmission) != 0 || len(server.runExecution) != 0 {
		t.Fatalf("canceled run leaked admission tokens: admission=%d execution=%d", len(server.runAdmission), len(server.runExecution))
	}
}

type protocolWiringExchange struct {
	client net.Conn
	result chan protocolWiringExchangeResult
	done   chan struct{}
}

type protocolWiringExchangeResult struct {
	response ResponseV1
	err      error
}

func startProtocolWiringExchange(t *testing.T, server *Server, id string) *protocolWiringExchange {
	t.Helper()
	client, serverConn := net.Pipe()
	exchange := &protocolWiringExchange{
		client: client,
		result: make(chan protocolWiringExchangeResult, 1),
		done:   make(chan struct{}),
	}
	t.Cleanup(func() {
		_ = client.Close()
		_ = serverConn.Close()
	})
	go func() {
		defer close(exchange.done)
		_ = server.serveConn(context.Background(), serverConn)
	}()
	go func() {
		err := EncodeRequestV1(client, RequestV1{
			Version: ProtocolVersionV1,
			Op:      OpRun,
			ID:      id,
			Run:     &RunRequestV1{Filter: id, Parallelism: 1},
		})
		if err != nil {
			exchange.result <- protocolWiringExchangeResult{err: err}
			return
		}
		response, err := DecodeResponseV1(bufio.NewReader(client))
		exchange.result <- protocolWiringExchangeResult{response: response, err: err}
	}()
	return exchange
}

func assertProtocolWiringBusyResponse(t *testing.T, result protocolWiringExchangeResult) {
	t.Helper()
	if result.err != nil {
		t.Fatalf("decode busy response: %v", result.err)
	}
	if result.response.Op != OpError || result.response.OK || result.response.Error != "test server is busy; retry the request" {
		t.Fatalf("busy response = %#v", result.response)
	}
}

func TestProtocolWiringV1ServerReadDeadlineRejectsSlowloris(t *testing.T) {
	previousTimeout := serverReadIOTimeoutV1
	serverReadIOTimeoutV1 = 100 * time.Millisecond
	defer func() { serverReadIOTimeoutV1 = previousTimeout }()
	server := protocolWiringReadyServer(t)
	client, serverConn := net.Pipe()
	defer client.Close()
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.serveConn(context.Background(), serverConn) }()
	if _, err := io.WriteString(client, `{"version":1,"op":"ping"`); err != nil {
		t.Fatal(err)
	}

	responseCh := make(chan struct {
		response ResponseV1
		err      error
	}, 1)
	go func() {
		response, err := DecodeResponseV1(bufio.NewReader(client))
		responseCh <- struct {
			response ResponseV1
			err      error
		}{response: response, err: err}
	}()
	select {
	case got := <-responseCh:
		if got.err != nil {
			t.Fatalf("decode slowloris rejection: %v", got.err)
		}
		if got.response.Op != OpError || got.response.OK || strings.TrimSpace(got.response.Error) == "" {
			t.Fatalf("slowloris response = %#v", got.response)
		}
	case <-time.After(time.Second):
		t.Fatal("server slowloris read did not reach its deadline")
	}
	if err := <-serveErr; err != nil {
		t.Fatalf("serve slowloris rejection: %v", err)
	}
}

func TestProtocolWiringV1ServerWriteDeadlineSurfacesNonreadingClient(t *testing.T) {
	previousTimeout := serverWriteIOTimeoutV1
	serverWriteIOTimeoutV1 = 100 * time.Millisecond
	defer func() { serverWriteIOTimeoutV1 = previousTimeout }()
	server := protocolWiringReadyServer(t)
	client, serverConn := net.Pipe()
	defer client.Close()
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.serveConn(context.Background(), serverConn) }()
	if err := EncodeRequestV1(client, RequestV1{Version: ProtocolVersionV1, Op: OpPing}); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-serveErr:
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "write") {
			t.Fatalf("nonreading client serve error = %v, want surfaced write failure", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server response write remained blocked beyond its deadline")
	}
}

func TestProtocolWiringV1OversizedResponseFallsBackToCompactError(t *testing.T) {
	server := protocolWiringReadyServer(t)
	server.daemon.mu.Lock()
	server.daemon.index.Dependencies = []typesys.DependencyInfo{{
		Namespace:  "oversized",
		SourceRoot: strings.Repeat("x", MaxProtocolFrameBytes),
	}}
	server.daemon.mu.Unlock()

	client, serverConn := net.Pipe()
	defer client.Close()
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.serveConn(context.Background(), serverConn) }()
	if err := EncodeRequestV1(client, RequestV1{
		Version: ProtocolVersionV1,
		Op:      OpRun,
		ID:      "oversized-response",
		Run: &RunRequestV1{
			Parallelism:       1,
			ShardCount:        1,
			ReturnClassShards: true,
		},
	}); err != nil {
		t.Fatal(err)
	}
	response, err := DecodeResponseV1(bufio.NewReader(client))
	if err != nil {
		t.Fatalf("decode compact oversized-response error: %v", err)
	}
	if response.Op != OpError || response.OK || response.ID != "oversized-response" || len(response.Error) == 0 || len(response.Error) > 1024 {
		t.Fatalf("oversized response fallback = %#v", response)
	}
	message := strings.ToLower(response.Error)
	if !strings.Contains(message, "response") || !strings.Contains(message, "maximum") {
		t.Fatalf("oversized response error = %q", response.Error)
	}
	if err := <-serveErr; err != nil {
		t.Fatalf("serve compact oversized-response error: %v", err)
	}
}

func TestProtocolWiringV1ServerLifecycleAndMismatch(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	close(ready)
	server := &Server{daemon: daemon, ready: true, warmDone: ready}

	ping := exchangeProtocolWiringV1(t, server, RequestV1{
		Version: ProtocolVersionV1,
		Op:      OpPing,
		ID:      "ping-1",
	})
	if ping.Version != ProtocolVersionV1 || ping.Op != OpPong || !ping.OK || ping.ID != "ping-1" {
		t.Fatalf("ping response = %#v", ping)
	}

	run := exchangeProtocolWiringV1(t, server, RequestV1{
		Version: ProtocolVersionV1,
		Op:      OpRun,
		ID:      "run-1",
		Run: &RunRequestV1{
			SelectedClasses: []string{"WarmTwoTest"},
			Parallelism:     1,
			ParallelMethods: false,
			TimeoutMS:       300_000,
		},
	})
	if run.Version != ProtocolVersionV1 || run.Op != OpRunResult || !run.OK || run.ID != "run-1" || run.Run == nil {
		t.Fatalf("run response = %#v", run)
	}
	if got := protocolWiringRunClasses(*run.Run); !reflect.DeepEqual(got, []string{"WarmTwoTest"}) {
		t.Fatalf("run classes = %#v, want WarmTwoTest only", got)
	}

	client, serverConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		server.serveConn(context.Background(), serverConn)
		close(done)
	}()
	if _, err := fmt.Fprintln(client, `{"version":2,"op":"run","id":"mismatch","run":{"parallelism":1}}`); err != nil {
		t.Fatal(err)
	}
	mismatch, err := DecodeResponseV1(bufio.NewReader(client))
	if err != nil {
		t.Fatalf("decode mismatch response: %v", err)
	}
	_ = client.Close()
	<-done
	if mismatch.Version != ProtocolVersionV1 || mismatch.Op != OpError || mismatch.OK {
		t.Fatalf("mismatch response = %#v", mismatch)
	}
	if !strings.Contains(strings.ToLower(mismatch.Error), "protocol version") {
		t.Fatalf("mismatch error = %q, want protocol version guidance", mismatch.Error)
	}

	shutdown := exchangeProtocolWiringV1(t, server, RequestV1{
		Version: ProtocolVersionV1,
		Op:      OpShutdown,
		ID:      "shutdown-1",
	})
	if shutdown.Version != ProtocolVersionV1 || shutdown.Op != OpShutdownAck || !shutdown.OK || shutdown.ID != "shutdown-1" {
		t.Fatalf("shutdown response = %#v", shutdown)
	}
}

func TestProtocolWiringV1ExactSelectorsReturnStructuredFailures(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	close(ready)
	server := &Server{daemon: daemon, ready: true, warmDone: ready}

	tests := []struct {
		name           string
		classes        []string
		method         string
		wantCaseName   string
		wantMessage    string
		wantDetailPart string
	}{
		{
			name:           "missing class",
			classes:        []string{"MissingTest"},
			wantCaseName:   "missing test class",
			wantMessage:    `no test class matched --class "MissingTest"`,
			wantDetailPart: `exact test class named "MissingTest"`,
		},
		{
			name:           "missing method",
			classes:        []string{"WarmOneTest"},
			method:         "missingMethod",
			wantCaseName:   "missing test method",
			wantMessage:    `no test method matched --class "WarmOneTest" --method "missingMethod"`,
			wantDetailPart: `no exact test method named "missingMethod"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := exchangeProtocolWiringV1(t, server, RequestV1{
				Version: ProtocolVersionV1,
				Op:      OpRun,
				Run: &RunRequestV1{
					SelectedClasses: test.classes,
					SelectedMethod:  test.method,
					Parallelism:     1,
				},
			})
			if !response.OK || response.Run == nil {
				t.Fatalf("selector response = %#v", response)
			}
			if got := response.Run.Summary(); got.Total != 1 || got.Errors != 1 {
				t.Fatalf("selector summary = %#v, want one structured error", got)
			}
			if len(response.Run.Suites) != 1 || len(response.Run.Suites[0].Cases) != 1 {
				t.Fatalf("selector run = %#v", response.Run)
			}
			got := response.Run.Suites[0].Cases[0]
			if got.Name != test.wantCaseName || got.Status != testreport.StatusRuntimeError || got.Problem == nil {
				t.Fatalf("selector case = %#v", got)
			}
			if got.Problem.Type != "Selector" || got.Problem.Message != test.wantMessage || !strings.Contains(got.Problem.Detail, test.wantDetailPart) {
				t.Fatalf("selector problem = %#v", got.Problem)
			}
		})
	}
}

func TestProtocolWiringV1ShardExecutionMatchesReturnedPlan(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	close(ready)
	server := &Server{daemon: daemon, ready: true, warmDone: ready}

	policy := RunRequestV1{
		Parallelism:       1,
		ClassDurationMS:   map[string]int64{"WarmOneTest": 20, "WarmTwoTest": 10},
		ShardCount:        2,
		ReturnClassShards: true,
	}
	planned := exchangeProtocolWiringV1(t, server, RequestV1{
		Version: ProtocolVersionV1,
		Op:      OpRun,
		Run:     &policy,
	})
	if !planned.OK || planned.ShardPlan == nil || len(planned.ShardPlan.Shards) != 2 {
		t.Fatalf("shard plan response = %#v", planned)
	}
	for _, shard := range planned.ShardPlan.Shards {
		shardPolicy := policy
		shardPolicy.ReturnClassShards = false
		shardPolicy.ShardIndex = shard.Index
		executed := exchangeProtocolWiringV1(t, server, RequestV1{
			Version: ProtocolVersionV1,
			Op:      OpRun,
			Run:     &shardPolicy,
		})
		if !executed.OK || executed.Run == nil {
			t.Fatalf("shard %d response = %#v", shard.Index, executed)
		}
		got := protocolWiringRunClasses(*executed.Run)
		want := append([]string(nil), shard.Classes...)
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("shard %d executed classes = %#v, want returned plan %#v", shard.Index, got, want)
		}
	}
}

func TestProtocolWiringV1ChangedSincePrecedesShardPlanningAndDependenciesAppearOnce(t *testing.T) {
	root := protocolWiringChangedSinceProject(t)
	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	wantDependency := typesys.DependencyInfo{
		Namespace:  "example",
		SourceRoot: "dependencies/example",
		Version:    "1.2.3",
		Status:     "loaded",
		ApexTypes:  3,
	}
	daemon.mu.Lock()
	daemon.index.Dependencies = []typesys.DependencyInfo{wantDependency}
	daemon.mu.Unlock()

	request := RunRequestV1{
		ChangedSince:      "HEAD",
		SelectedClasses:   []string{"ContactServiceTest", "BillingServiceTest", "UnchangedServiceTest"},
		Parallelism:       1,
		ClassDurationMS:   map[string]int64{"BillingServiceTest": 900, "ContactServiceTest": 800, "UnchangedServiceTest": 10_000},
		ShardCount:        2,
		ShardIndex:        0,
		ReturnClassShards: true,
	}
	run, selection, plan, err := daemon.runRequestV1(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || len(plan.Shards) != 2 {
		t.Fatalf("shard plan = %#v, want two shards", plan)
	}
	var planned []string
	for _, shard := range plan.Shards {
		planned = append(planned, shard.Classes...)
	}
	sort.Strings(planned)
	if want := []string{"BillingServiceTest", "ContactServiceTest"}; !reflect.DeepEqual(planned, want) {
		t.Fatalf("planned classes = %#v, want affected classes %#v", planned, want)
	}
	if got := selection.TestClasses; !reflect.DeepEqual(got, []string{"BillingServiceTest", "ContactServiceTest"}) {
		t.Fatalf("affected selection = %#v", got)
	}
	if got := run.Dependencies; !reflect.DeepEqual(got, []typesys.DependencyInfo{wantDependency}) {
		t.Fatalf("run dependencies = %#v, want exactly one %#v", got, wantDependency)
	}
	if protocolWiringRunHasClass(run, "UnchangedServiceTest") {
		t.Fatalf("run included unaffected class: %#v", run)
	}
}

func TestProtocolWiringV1CanceledDaemonRequestStopsBeforeGitDiscoveryAndPlanning(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	run, selection, plan, err := daemon.runRequestV1(ctx, RunRequestV1{
		ChangedSince:      "HEAD",
		Parallelism:       1,
		ShardCount:        2,
		ReturnClassShards: true,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("already-canceled daemon request: err=%v, want context.Canceled", err)
	}
	if plan != nil {
		t.Fatalf("already-canceled daemon request returned shard plan: %#v", plan)
	}
	if run.Summary().Total != 0 || len(selection.TestClasses) != 0 {
		t.Fatalf("already-canceled daemon request produced work: run=%#v selection=%#v", run, selection)
	}
}

func exchangeProtocolWiringV1(t *testing.T, server *Server, request RequestV1) ResponseV1 {
	t.Helper()
	client, serverConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		server.serveConn(context.Background(), serverConn)
		close(done)
	}()
	if err := EncodeRequestV1(client, request); err != nil {
		t.Fatalf("encode request: %v", err)
	}
	response, err := DecodeResponseV1(bufio.NewReader(client))
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	_ = client.Close()
	<-done
	return response
}

func protocolWiringReadyServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	writeTestProject(t, root)
	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	close(ready)
	return &Server{daemon: daemon, ready: true, warmDone: ready}
}

func protocolWiringUnixListener(t *testing.T) (string, net.Listener) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "glade-pv1-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "p.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	return socket, listener
}

func distinctProtocolWiringLimitCaps() vm.LimitCaps {
	return vm.LimitCaps{
		Queries:             1,
		QueryRows:           2,
		DMLStatements:       3,
		DMLRows:             4,
		HeapSize:            5,
		CPUTimeMS:           6,
		Callouts:            7,
		AsyncJobs:           8,
		FutureCalls:         9,
		QueueableJobs:       10,
		BatchJobs:           11,
		ScheduledJobs:       12,
		EmailInvokes:        13,
		SOSLQueries:         14,
		QueryLocatorRows:    15,
		RunAs:               16,
		Savepoints:          17,
		SavepointRollbacks:  18,
		PublishImmediateDML: 19,
	}
}

func distinctProtocolWiringLimitCapsV1() LimitCapsV1 {
	return LimitCapsV1{
		Queries:             1,
		QueryRows:           2,
		DMLStatements:       3,
		DMLRows:             4,
		HeapSize:            5,
		CPUTimeMS:           6,
		Callouts:            7,
		AsyncJobs:           8,
		FutureCalls:         9,
		QueueableJobs:       10,
		BatchJobs:           11,
		ScheduledJobs:       12,
		EmailInvokes:        13,
		SOSLQueries:         14,
		QueryLocatorRows:    15,
		RunAs:               16,
		Savepoints:          17,
		SavepointRollbacks:  18,
		PublishImmediateDML: 19,
	}
}

func protocolWiringRunClasses(run testreport.Run) []string {
	var classes []string
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			classes = append(classes, testCase.ClassName)
		}
	}
	return classes
}

func protocolWiringRunHasClass(run testreport.Run, className string) bool {
	for _, got := range protocolWiringRunClasses(run) {
		if got == className {
			return true
		}
	}
	return false
}

func protocolWiringChangedSinceProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	for _, name := range []string{"Billing", "Contact", "Unchanged"} {
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+"Service.cls"), fmt.Sprintf(`
public class %[1]sService {
  public static Integer value() { return 1; }
}
`, name))
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+"ServiceTest.cls"), fmt.Sprintf(`
@isTest
private class %[1]sServiceTest {
  @isTest static void passes() { System.assertEquals(1, %[1]sService.value()); }
}
`, name))
	}
	protocolWiringGit(t, root, "init")
	protocolWiringGit(t, root, "config", "user.email", "tests@glade.local")
	protocolWiringGit(t, root, "config", "user.name", "Glade Tests")
	protocolWiringGit(t, root, "add", ".")
	protocolWiringGit(t, root, "commit", "-m", "baseline")
	for _, name := range []string{"Billing", "Contact"} {
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+"Service.cls"), fmt.Sprintf(`
public class %[1]sService {
  public static Integer value() { Integer changed = 1; return changed; }
}
`, name))
	}
	return root
}

func protocolWiringGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
