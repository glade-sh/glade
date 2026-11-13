package gladecli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/lsp"
	"path/filepath"
	"testing"
)

func TestDiagnosticsRefreshAfterSave(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	project := filepath.Join(root, "project")
	writeTestProject(t, project)
	file := filepath.Join(project, "force-app/main/classes/Refresh.cls")
	writeTestFile(t, file, "public class Refresh { public void run() {} }")
	l, err := newLSPProjectLoader(project)
	if err != nil {
		t.Fatal(err)
	}
	l.analyzer(l.artifacts)(l.index)
	writeTestFile(t, file, "public class Refresh { public void run() { Integer x = missing; } }")
	index, analyze, err := l.refresh(context.Background(), []string{file})
	if err != nil {
		t.Fatal(err)
	}
	broken := analyze(index)
	if len(broken.Diagnostics) == 0 {
		t.Fatal("save did not produce diagnostics")
	}
	writeTestFile(t, file, "public class Refresh { public void run() {} }")
	index, analyze, err = l.refresh(context.Background(), []string{file})
	if err != nil {
		t.Fatal(err)
	}
	fixed := analyze(index)
	if len(fixed.Diagnostics) >= len(broken.Diagnostics) {
		t.Fatalf("diagnostics not cleared: %#v", fixed.Diagnostics)
	}
}

func TestSaveRefreshIsIncremental(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	project := filepath.Join(root, "project")
	writeTestProject(t, project)
	file := filepath.Join(project, "force-app/main/classes/One.cls")
	writeTestFile(t, file, "public class One { public void run() { Integer x = 1; } }")
	writeTestFile(t, filepath.Join(project, "force-app/main/classes/Two.cls"), "public class Two { public void run() { Integer x = 1; } }")
	l, err := newLSPProjectLoader(project)
	if err != nil {
		t.Fatal(err)
	}
	l.analyzer(l.artifacts)(l.index)
	for _, n := range []string{"2", "3"} {
		writeTestFile(t, file, "public class One { public void run() { Integer x = "+n+"; } }")
		index, analyze, err := l.refresh(context.Background(), []string{file})
		if err != nil {
			t.Fatal(err)
		}
		analyze(index)
	}
	stats := l.state.LastStats()
	if !stats.Incremental || stats.Rechecked >= stats.Types {
		t.Fatalf("not incremental: %+v", stats)
	}
}

func TestSaveRefreshPublishesDependentsAndSymbols(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	project := filepath.Join(root, "project")
	writeTestProject(t, project)
	a := filepath.Join(project, "force-app/main/classes/A.cls")
	b := filepath.Join(project, "force-app/main/classes/B.cls")
	initial := "public class A { public static void run() {} }"
	writeTestFile(t, a, initial)
	writeTestFile(t, b, "public class B { public void test() { A.run(); } }")
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = inR
	t.Cleanup(func() { os.Stdin = old; inR.Close(); inW.Close() })
	outR, outW := io.Pipe()
	c := &lspClient{t: t, in: inW, start: time.Now()}
	c.cond = sync.NewCond(&c.mu)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { done <- runLSP(ctx, []string{"--project", project}, outW); outW.Close() }()
	go func() {
		reader := lsp.NewProtocolReader(outR)
		for {
			data, err := reader.Read()
			if err != nil {
				return
			}
			var m map[string]any
			if json.Unmarshal(data, &m) != nil {
				continue
			}
			c.mu.Lock()
			c.seen = append(c.seen, m)
			c.cond.Broadcast()
			c.mu.Unlock()
		}
	}()
	uriA, uriB := "file://"+a, "file://"+b
	c.send(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"capabilities": map[string]any{}}})
	response := func(id int) func(map[string]any) bool {
		return func(m map[string]any) bool { return m["id"] == float64(id) }
	}
	c.wait("initialize", 30*time.Second, response(1))
	c.send(map[string]any{"method": "initialized", "params": map[string]any{}})
	c.send(map[string]any{"method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": uriA, "languageId": "apex", "version": 1, "text": initial}}})
	c.wait("initial diagnostics", 30*time.Second, func(m map[string]any) bool {
		p, _ := m["params"].(map[string]any)
		return m["method"] == "textDocument/publishDiagnostics" && p["uri"] == uriA
	})
	for i, body := range []string{"public class A { public static void replacement() {} }", initial} {
		writeTestFile(t, a, body)
		c.mu.Lock()
		start := len(c.seen)
		c.mu.Unlock()
		c.send(map[string]any{"method": "textDocument/didSave", "params": map[string]any{"textDocument": map[string]any{"uri": uriA}}})
		c.waitAfter("dependent diagnostics", 30*time.Second, start, func(m map[string]any) bool {
			p, _ := m["params"].(map[string]any)
			d, _ := p["diagnostics"].([]any)
			return m["method"] == "textDocument/publishDiagnostics" && p["uri"] == uriB && ((i == 0 && len(d) > 0) || (i == 1 && len(d) == 0))
		})
		if i == 0 {
			c.send(map[string]any{"id": 4, "method": "textDocument/documentSymbol", "params": map[string]any{"textDocument": map[string]any{"uri": uriA}}})
			c.wait("new symbols", 30*time.Second, response(4))
			c.mu.Lock()
			found := false
			for _, m := range c.seen {
				if response(4)(m) {
					raw, _ := json.Marshal(m["result"])
					found = strings.Contains(string(raw), "replacement")
				}
			}
			c.mu.Unlock()
			if !found {
				t.Fatal("added method missing after save")
			}
		}

	}
	c.send(map[string]any{"id": 2, "method": "textDocument/documentSymbol", "params": map[string]any{"textDocument": map[string]any{"uri": uriA}}})
	c.wait("symbols", 30*time.Second, response(2))
	c.mu.Lock()
	found := false
	for _, m := range c.seen {
		if response(2)(m) {
			raw, _ := json.Marshal(m["result"])
			found = strings.Contains(string(raw), "run")
		}
	}
	c.mu.Unlock()
	if !found {
		t.Fatal("saved symbols not refreshed")
	}
	c.send(map[string]any{"id": 3, "method": "shutdown"})
	c.wait("shutdown", 30*time.Second, response(3))
	c.send(map[string]any{"method": "exit"})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRefreshRemovesDeletedSource(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	project := filepath.Join(root, "project")
	writeTestProject(t, project)
	file := filepath.Join(project, "force-app/main/classes/Deleted.cls")
	writeTestFile(t, file, "public class Deleted {}")
	l, err := newLSPProjectLoader(project)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	index, _, err := l.refresh(context.Background(), []string{file})
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range index.Types {
		if typ.Name == "Deleted" {
			t.Fatal("deleted type remains in index")
		}
	}
}
