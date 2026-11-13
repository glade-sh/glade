package gladecli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/dap"
	"github.com/glade-sh/glade/internal/lsp"
)

// TestEditorLatencyHarness measures the editor-facing latency of the language
// server and the debug adapter against a generated project. Without
// GLADE_EDITOR_LATENCY it runs a tiny project as a smoke check and asserts no
// timing. With GLADE_EDITOR_LATENCY=<classes> it generates that many classes
// and logs one "latency <name> <duration>" line per measurement:
//
//	GLADE_EDITOR_LATENCY=600 go test ./internal/gladecli -run TestEditorLatencyHarness -v -count=1
//
// Each run starts from an empty data home, so the numbers are cold-cache.
func TestEditorLatencyHarness(t *testing.T) {
	classes, report := 4, false
	if raw := strings.TrimSpace(os.Getenv("GLADE_EDITOR_LATENCY")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			t.Fatalf("GLADE_EDITOR_LATENCY = %q, want a class count", raw)
		}
		classes, report = n, true
	}
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "xdgcache"))
	project := filepath.Join(root, "project")
	writeGeneratedLatencyProject(t, project, classes)

	record := func(name string, d time.Duration) {
		if report {
			t.Logf("latency %s %s", name, d.Round(time.Millisecond))
		}
	}
	t.Run("lsp", func(t *testing.T) { measureLSPLatency(t, project, record) })
	t.Run("dap_cancel_during_launch", func(t *testing.T) { measureDAPCancelLatency(t, project, record) })
}

func writeGeneratedLatencyProject(t *testing.T, root string, classes int) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	for i := 0; i < classes; i++ {
		var b strings.Builder
		fmt.Fprintf(&b, "public with sharing class LatencyService%04d {\n", i)
		b.WriteString("  public String label;\n")
		for m := 0; m < 25; m++ {
			fmt.Fprintf(&b, "  public static Integer step%02d(Integer value) {\n    Integer total = value + %d;\n    List<Integer> items = new List<Integer>{ total, value };\n    for (Integer item : items) { total += item; }\n    return total;\n  }\n", m, m)
		}
		if i > 0 {
			fmt.Fprintf(&b, "  public static Integer chain(Integer value) { return LatencyService%04d.step00(value); }\n", i-1)
		}
		b.WriteString("}\n")
		writeTestFile(t, filepath.Join(root, fmt.Sprintf("force-app/main/classes/LatencyService%04d.cls", i)), b.String())
	}
}

type lspClient struct {
	t     *testing.T
	in    *os.File
	start time.Time
	mu    sync.Mutex
	seen  []map[string]any
	cond  *sync.Cond
}

func (c *lspClient) send(message map[string]any) {
	c.t.Helper()
	message["jsonrpc"] = "2.0"
	if err := lsp.WriteMessage(c.in, message); err != nil {
		c.t.Fatalf("write %v: %v", message["method"], err)
	}
}

// wait blocks until a message matching match arrives and returns the time since
// the client started.
func (c *lspClient) wait(what string, limit time.Duration, match func(map[string]any) bool) time.Duration {
	c.t.Helper()
	deadline := time.Now().Add(limit)
	timer := time.AfterFunc(limit, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.cond.Broadcast()
	})
	defer timer.Stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	next := 0
	for {
		for ; next < len(c.seen); next++ {
			if match(c.seen[next]) {
				return time.Since(c.start)
			}
		}
		if time.Now().After(deadline) {
			c.t.Fatalf("timed out waiting for %s", what)
		}
		c.cond.Wait()
	}
}

func measureLSPLatency(t *testing.T, project string, record func(string, time.Duration)) {
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	realStdin := os.Stdin
	os.Stdin = stdinR
	t.Cleanup(func() { os.Stdin = realStdin })

	outR, outW := io.Pipe()
	client := &lspClient{t: t, in: stdinW}
	client.cond = sync.NewCond(&client.mu)
	done := make(chan error, 1)
	client.start = time.Now()
	go func() {
		err := runLSP(context.Background(), []string{"--project", project}, outW)
		_ = outW.Close()
		done <- err
	}()
	go func() {
		reader := lsp.NewProtocolReader(outR)
		for {
			data, err := reader.Read()
			if err != nil {
				return
			}
			var message map[string]any
			if json.Unmarshal(data, &message) != nil {
				continue
			}
			client.mu.Lock()
			client.seen = append(client.seen, message)
			client.mu.Unlock()
			client.cond.Broadcast()
		}
	}()
	isResponse := func(id int) func(map[string]any) bool {
		return func(m map[string]any) bool {
			got, ok := m["id"].(float64)
			return ok && int(got) == id && m["method"] == nil
		}
	}
	limit := 3 * time.Minute

	client.send(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"processId": nil, "rootUri": "file://" + project, "capabilities": map[string]any{}}})
	record("lsp_initialize", client.wait("initialize", limit, isResponse(1)))
	client.send(map[string]any{"method": "initialized", "params": map[string]any{}})

	symbolStart := time.Now()
	client.send(map[string]any{"id": 2, "method": "workspace/symbol", "params": map[string]any{"query": "LatencyService0000"}})
	client.wait("workspace/symbol", limit, isResponse(2))
	record("lsp_first_symbol_request", time.Since(symbolStart))

	file := filepath.Join(project, "force-app/main/classes/LatencyService0000.cls")
	text, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	uri := "file://" + file
	openStart := time.Now()
	client.send(map[string]any{"method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "apex", "version": 1, "text": string(text)}}})
	hoverStart := time.Now()
	client.send(map[string]any{"id": 4, "method": "textDocument/hover", "params": map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]any{"line": 1, "character": 16}}})
	client.wait("hover", limit, isResponse(4))
	record("lsp_first_hover", time.Since(hoverStart))
	client.wait("diagnostics", limit, func(m map[string]any) bool {
		if m["method"] != "textDocument/publishDiagnostics" {
			return false
		}
		params, _ := m["params"].(map[string]any)
		return params["uri"] == uri
	})
	record("lsp_first_didopen_diagnostics", time.Since(openStart))

	client.send(map[string]any{"id": 3, "method": "shutdown"})
	client.wait("shutdown", limit, isResponse(3))
	client.send(map[string]any{"method": "exit"})
	_ = stdinW.Close()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("language server did not stop after exit")
	}
}

func measureDAPCancelLatency(t *testing.T, project string, record func(string, time.Duration)) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := runDAP(context.Background(), []string{"--project", project}, inR, outW)
		_ = outW.Close()
		done <- err
	}()
	send := func(seq int, command string, args any) {
		data, err := encodeDAPRequest(command, seq, args)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := inW.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	responses := map[string]time.Time{}
	arrived := make(chan struct{}, 16)
	go func() {
		reader := dap.NewReader(outR)
		for {
			raw, err := reader.Read()
			if err != nil {
				return
			}
			var message struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			}
			if json.Unmarshal(raw, &message) == nil && message.Type == dap.MessageTypeResponse {
				mu.Lock()
				responses[message.Command] = time.Now()
				mu.Unlock()
				select {
				case arrived <- struct{}{}:
				default:
				}
			}
		}
	}()
	send(1, "initialize", map[string]any{"adapterID": "glade"})
	deadline := time.After(2 * time.Minute)
	for {
		mu.Lock()
		_, ok := responses["initialize"]
		mu.Unlock()
		if ok {
			break
		}
		select {
		case <-arrived:
		case <-deadline:
			t.Fatal("no initialize response")
		}
	}
	send(2, "launch", map[string]any{"anonymousBody": "System.debug(LatencyService0000.step00(1));", "project": project})
	time.Sleep(time.Second)
	disconnectAt := time.Now()
	send(3, "disconnect", nil)
	for {
		mu.Lock()
		at, ok := responses["disconnect"]
		mu.Unlock()
		if ok {
			record("dap_disconnect_during_launch", at.Sub(disconnectAt))
			break
		}
		select {
		case <-arrived:
		case <-deadline:
			t.Fatal("no disconnect response")
		}
	}
	_ = inW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Minute):
		t.Fatal("debug adapter did not stop after disconnect")
	}
}
