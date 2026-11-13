package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestShutdownDoesNotParseAfterParserClose(t *testing.T) {
	var parses atomic.Int64
	previous := apexast.SetSourceObserver(func(string) { parses.Add(1) })
	t.Cleanup(func() { apexast.SetSourceObserver(previous) })

	handler := NewHandler(sampleIndex(t))
	if _, rpcErr := handler.handle("shutdown", nil); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	before := parses.Load()
	uri := uriFromPath(filepath.Join(t.TempDir(), "Draft.cls"))
	handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Text: "public class Draft {\n"}})
	if got := parses.Load(); got != before {
		t.Fatalf("parser used after Close: %d parse calls", got-before)
	}
}

func TestServeShutdownStopsPendingDiagnosticsBeforeClose(t *testing.T) {
	var parses atomic.Int64
	previous := apexast.SetSourceObserver(func(string) { parses.Add(1) })
	t.Cleanup(func() { apexast.SetSourceObserver(previous) })

	handler := NewHandler(sampleIndex(t))
	handler.SetDiagnosticsDebounce(time.Hour)
	c := startServe(t, handler)
	uri := uriFromPath(filepath.Join(t.TempDir(), "Draft.cls"))
	c.send(openMessage(uri, "public class Draft {}\n"))
	c.mustNext(2 * time.Second)
	c.send(changeMessage(uri, 2, "public class Draft {\n"))
	c.send(map[string]any{"id": 9, "method": "shutdown"})
	if err := <-c.served; err != nil {
		t.Fatal(err)
	}
	settled := parses.Load()
	time.Sleep(50 * time.Millisecond)
	if parses.Load() != settled {
		t.Fatal("diagnostics ran after Serve returned")
	}
}

type serveClient struct {
	t      *testing.T
	stdin  *io.PipeWriter
	out    chan json.RawMessage
	served chan error
}

func startServe(t *testing.T, handler *Handler) *serveClient {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := &serveClient{t: t, stdin: inW, out: make(chan json.RawMessage, 64), served: make(chan error, 1)}
	go func() {
		err := Serve(inR, outW, handler)
		outW.Close()
		c.served <- err
	}()
	go func() {
		reader := NewProtocolReader(outR)
		for {
			data, err := reader.Read()
			if err != nil {
				close(c.out)
				return
			}
			c.out <- data
		}
	}()
	t.Cleanup(func() { inW.Close() })
	return c
}

func (c *serveClient) send(message map[string]any) {
	c.t.Helper()
	message["jsonrpc"] = "2.0"
	if err := WriteMessage(c.stdin, message); err != nil {
		c.t.Fatal(err)
	}
}

func (c *serveClient) next(timeout time.Duration) (map[string]json.RawMessage, bool) {
	c.t.Helper()
	select {
	case data, ok := <-c.out:
		if !ok {
			return nil, false
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(data, &m); err != nil {
			c.t.Fatal(err)
		}
		return m, true
	case <-time.After(timeout):
		return nil, false
	}
}

func (c *serveClient) mustNext(timeout time.Duration) map[string]json.RawMessage {
	c.t.Helper()
	m, ok := c.next(timeout)
	if !ok {
		c.t.Fatal("no message before timeout")
	}
	return m
}

func syntheticIndex(types int) typesys.Index {
	index := typesys.Index{}
	for i := 0; i < types; i++ {
		index.Types = append(index.Types, typesys.TypeSymbol{Name: fmt.Sprintf("Synthetic%d", i), File: fmt.Sprintf("/p/Synthetic%d.cls", i)})
	}
	return index
}

func TestServeInitializeDoesNotWaitForIndex(t *testing.T) {
	handler := NewPendingHandler()
	var indexed atomic.Bool
	release := make(chan struct{})
	go func() {
		index := syntheticIndex(20000)
		<-release
		indexed.Store(true)
		handler.SetIndex(index)
	}()
	c := startServe(t, handler)
	start := time.Now()
	c.send(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{}})
	resp := c.mustNext(2 * time.Second)
	if string(resp["id"]) != "1" || resp["error"] != nil {
		t.Fatalf("initialize response = %v", resp)
	}
	if indexed.Load() {
		t.Fatal("index finished before initialize answered; test proves nothing")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("initialize took %s", elapsed)
	}

	// A request that needs the index waits, then answers once it is ready.
	c.send(map[string]any{"id": 2, "method": "workspace/symbol", "params": map[string]any{"query": "Synthetic19999"}})
	if m, ok := c.next(150 * time.Millisecond); ok {
		t.Fatalf("workspace/symbol answered before the index was ready: %v", m)
	}
	close(release)
	resp = c.mustNext(2 * time.Second)
	if string(resp["id"]) != "2" || resp["error"] != nil {
		t.Fatalf("workspace/symbol response = %v", resp)
	}
	var symbols []SymbolInformation
	if err := json.Unmarshal(resp["result"], &symbols); err != nil || len(symbols) != 1 {
		t.Fatalf("symbols = %v err=%v", symbols, err)
	}
}

func TestServeCancelRequestEndsWaitingRequest(t *testing.T) {
	handler := NewPendingHandler() // index never arrives
	c := startServe(t, handler)
	c.send(map[string]any{"id": 7, "method": "workspace/symbol", "params": map[string]any{"query": "x"}})
	if m, ok := c.next(100 * time.Millisecond); ok {
		t.Fatalf("unexpected message %v", m)
	}
	c.send(map[string]any{"method": "$/cancelRequest", "params": map[string]any{"id": 7}})
	resp := c.mustNext(2 * time.Second)
	var rpcErr ResponseError
	if err := json.Unmarshal(resp["error"], &rpcErr); err != nil || rpcErr.Code != errorCodeRequestCancelled {
		t.Fatalf("response = %v", resp)
	}
	if string(resp["id"]) != "7" {
		t.Fatalf("id = %s", resp["id"])
	}
}

func TestServeIndexFailureAnswersWaitingRequest(t *testing.T) {
	handler := NewPendingHandler()
	c := startServe(t, handler)
	c.send(map[string]any{"id": 3, "method": "workspace/symbol", "params": map[string]any{"query": "x"}})
	handler.Fail(fmt.Errorf("boom"))
	resp := c.mustNext(2 * time.Second)
	var rpcErr ResponseError
	if err := json.Unmarshal(resp["error"], &rpcErr); err != nil || rpcErr.Code != errorCodeInternalError {
		t.Fatalf("response = %v", resp)
	}
	if handler.LoadError() == nil {
		t.Fatal("LoadError lost")
	}
}

func openMessage(uri DocumentURI, text string) map[string]any {
	return map[string]any{"method": "textDocument/didOpen", "params": map[string]any{
		"textDocument": map[string]any{"uri": uri, "version": 1, "text": text},
	}}
}

func changeMessage(uri DocumentURI, version int, text string) map[string]any {
	return map[string]any{"method": "textDocument/didChange", "params": map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": version},
		"contentChanges": []map[string]any{{"text": text}},
	}}
}

func diagnosticCount(t *testing.T, m map[string]json.RawMessage) int {
	t.Helper()
	var method string
	_ = json.Unmarshal(m["method"], &method)
	if method != "textDocument/publishDiagnostics" {
		t.Fatalf("unexpected message %v", m)
	}
	var p PublishDiagnosticsParams
	if err := json.Unmarshal(m["params"], &p); err != nil {
		t.Fatal(err)
	}
	return len(p.Diagnostics)
}

func TestServeDidChangeDebouncesAndKeepsFinalState(t *testing.T) {
	handler := NewHandler(sampleIndex(t))
	handler.SetDiagnosticsDebounce(150 * time.Millisecond)
	c := startServe(t, handler)
	uri := uriFromPath(filepath.Join(t.TempDir(), "Draft.cls"))
	c.send(openMessage(uri, "public class Draft {}\n"))
	if n := diagnosticCount(t, c.mustNext(2*time.Second)); n != 0 {
		t.Fatalf("open diagnostics = %d", n)
	}
	for i := 2; i <= 6; i++ {
		c.send(changeMessage(uri, i, "public class Draft {}\n"))
	}
	c.send(changeMessage(uri, 7, "public class Draft {\n"))
	if m, ok := c.next(60 * time.Millisecond); ok {
		t.Fatalf("diagnostics published inside the debounce window: %v", m)
	}
	first := c.mustNext(2 * time.Second)
	if n := diagnosticCount(t, first); n == 0 {
		t.Fatal("final broken text produced no diagnostics")
	}
	if m, ok := c.next(400 * time.Millisecond); ok {
		t.Fatalf("burst published more than once: %v", m)
	}
}

func TestServeEOFFlushesPendingDebounce(t *testing.T) {
	handler := NewHandler(sampleIndex(t))
	handler.SetDiagnosticsDebounce(time.Hour)
	c := startServe(t, handler)
	uri := uriFromPath(filepath.Join(t.TempDir(), "Draft.cls"))
	c.send(openMessage(uri, "public class Draft {}\n"))
	c.mustNext(2 * time.Second)
	c.send(changeMessage(uri, 2, "public class Draft {\n"))
	c.stdin.Close()
	if n := diagnosticCount(t, c.mustNext(2*time.Second)); n == 0 {
		t.Fatal("pending change was dropped at EOF")
	}
	if err := <-c.served; err != nil {
		t.Fatal(err)
	}
}

func TestServeDidOpenWithPendingIndexPublishesParseErrorsAtOnce(t *testing.T) {
	handler := NewPendingHandler()
	c := startServe(t, handler)
	uri := uriFromPath(filepath.Join(t.TempDir(), "Draft.cls"))
	c.send(openMessage(uri, "public class Draft {\n"))
	if n := diagnosticCount(t, c.mustNext(2*time.Second)); n == 0 {
		t.Fatal("expected parse diagnostics without the index")
	}
}

func TestServeIgnoresClientResponses(t *testing.T) {
	c := startServe(t, NewHandler(sampleIndex(t)))
	c.send(map[string]any{"id": 42, "result": nil})
	c.send(map[string]any{"id": 43, "error": map[string]any{"code": -32601, "message": "unsupported"}})
	c.send(map[string]any{"id": 44, "method": "initialize", "params": map[string]any{}})
	m := c.mustNext(3 * time.Second)
	if string(m["id"]) != "44" {
		t.Fatalf("response to client response: %s", m["id"])
	}
	c.send(map[string]any{"id": 45, "method": "shutdown"})
	if err := <-c.served; err != nil {
		t.Fatal(err)
	}
}

func TestDiagnosticsRetriesAfterGenerationChange(t *testing.T) {
	idx := sampleIndex(t)
	h := NewHandler(idx)
	started, release := make(chan struct{}), make(chan struct{})
	h.gen.Load().analyzer = func(typesys.Index) sema.Result { close(started); <-release; return sema.Result{} }
	uri := uriFromPath(filepath.Join(t.TempDir(), "Draft.cls"))
	h.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Text: "public class Draft {}"}})
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &server{h: h, w: &out, ctx: ctx, pending: make(map[DocumentURI]*diagJob)}
	s.schedule(uri, 0)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("analysis did not start")
	}
	h.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Text: "public class Draft {"}})
	s.pubMu.Lock()
	h.gen.Store(newGeneration(idx, 1, func(typesys.Index) sema.Result { return sema.Result{} }))
	s.pubMu.Unlock()
	close(release)
	s.diagWG.Wait()
	data, err := NewProtocolReader(&out).Read()
	if err != nil {
		t.Fatalf("diagnostics not published after refresh: %v", err)
	}
	var n Notification
	if err := json.Unmarshal(data, &n); err != nil || n.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("notification = %s, error = %v", data, err)
	}
}
