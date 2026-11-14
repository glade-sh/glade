package gladecli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/lsp"
)

func TestClosedExternalParseDiagnosticsAndRecovery(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	project := filepath.Join(root, "project")
	writeTestProject(t, project)
	broken := filepath.Join(project, "force-app/main/classes/First.cls")
	other := filepath.Join(project, "force-app/main/classes/Second.cls")
	initial := "public class First { public static void run() {} }"
	writeTestFile(t, broken, initial)
	writeTestFile(t, other, "public class Second { public void keep() { First.run(); } }")
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdin
	os.Stdin = inR
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); inR.Close(); inW.Close(); outR.Close(); os.Stdin = original })
	c := &lspClient{t: t, in: inW, start: time.Now()}
	c.cond = sync.NewCond(&c.mu)
	done := make(chan error, 1)
	go func() { done <- runLSP(ctx, []string{"--project", project}, outW); outW.Close() }()
	go func() {
		reader := lsp.NewProtocolReader(outR)
		for {
			data, err := reader.Read()
			if err != nil {
				return
			}
			var m map[string]any
			if json.Unmarshal(data, &m) == nil {
				c.mu.Lock()
				c.seen = append(c.seen, m)
				c.cond.Broadcast()
				c.mu.Unlock()
			}
		}
	}()
	response := func(id int) func(map[string]any) bool {
		return func(m map[string]any) bool { return m["id"] == float64(id) }
	}
	uri := "file://" + broken
	c.send(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"capabilities": map[string]any{}}})
	c.wait("initialize", 30*time.Second, response(1))
	c.send(map[string]any{"id": 2, "method": "workspace/symbol", "params": map[string]any{"query": "Second"}})
	c.wait("initial index", 30*time.Second, response(2))
	writeTestFile(t, broken, "public class First {")
	c.send(map[string]any{"method": "workspace/didChangeWatchedFiles", "params": map[string]any{"changes": []any{map[string]any{"uri": uri, "type": 2}}}})
	matches := func(nonempty bool) func(map[string]any) bool {
		return func(m map[string]any) bool {
			p, _ := m["params"].(map[string]any)
			d, _ := p["diagnostics"].([]any)
			return m["method"] == "textDocument/publishDiagnostics" && p["uri"] == uri && (len(d) > 0) == nonempty
		}
	}
	c.wait("closed-file parse diagnostics", 10*time.Second, matches(true))
	c.send(map[string]any{"id": 3, "method": "workspace/symbol", "params": map[string]any{"query": "Second"}})
	c.wait("last good symbols", 30*time.Second, response(3))
	c.mu.Lock()
	found := false
	for _, m := range c.seen {
		if response(3)(m) {
			raw, _ := json.Marshal(m["result"])
			found = strings.Contains(string(raw), "Second")
		}
	}
	start := len(c.seen)
	c.mu.Unlock()
	if !found {
		t.Fatal("other file lost its last good symbols")
	}
	writeTestFile(t, broken, initial)
	c.send(map[string]any{"method": "workspace/didChangeWatchedFiles", "params": map[string]any{"changes": []any{map[string]any{"uri": uri, "type": 2}}}})
	c.waitAfter("cleared parse diagnostics", 10*time.Second, start, matches(false))
	c.send(map[string]any{"id": 4, "method": "shutdown"})
	c.wait("shutdown", 30*time.Second, response(4))
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
