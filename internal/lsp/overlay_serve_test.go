package lsp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// overlayServeProject writes an sfdx project and returns its loaded project
// and the URI of each class.
func overlayServeProject(t *testing.T, sources map[string]string) (project.Project, map[string]DocumentURI) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"classes","default":true}],"sourceApiVersion":"67.0"}`), 0644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "classes")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	uris := map[string]DocumentURI{}
	for name, text := range sources {
		path := filepath.Join(dir, name+".cls")
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
		uris[name] = uriFromPath(path)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return p, uris
}

func overlayServeHandler(t *testing.T, p project.Project, analyze func(typesys.Index, *typesys.BuildArtifacts) sema.Result) *Handler {
	t.Helper()
	h := NewPendingHandler()
	h.SetDiagnosticsDebounce(10 * time.Millisecond)
	h.SetBase(Base{Index: typesys.Build(p, schema.Schema{}), Project: p, AnalyzeOverlay: analyze})
	return h
}

func plainOverlayAnalysis(i typesys.Index, a *typesys.BuildArtifacts) sema.Result {
	return sema.AnalyzeWithOptions(i, sema.AnalyzeOptions{Diagnostics: true, ExportTypes: true, BuildArtifacts: a})
}

// publishCount decodes a publishDiagnostics message for uri. ok is false for
// any other message.
func publishCount(t *testing.T, m map[string]json.RawMessage, uri DocumentURI) (int, bool) {
	t.Helper()
	var method string
	_ = json.Unmarshal(m["method"], &method)
	if method != "textDocument/publishDiagnostics" {
		return 0, false
	}
	var p PublishDiagnosticsParams
	if err := json.Unmarshal(m["params"], &p); err != nil {
		t.Fatal(err)
	}
	return len(p.Diagnostics), p.URI == uri
}

// While an analysis of an older text is held, a newer edit arrives. No
// publish may carry the older text's semantic diagnostics.
func TestStaleSemanticDiagnosticsAreNotPublished(t *testing.T) {
	initial := "public class First { void run() { Integer x = 1; } }"
	p, uris := overlayServeProject(t, map[string]string{"First": initial})
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	var firstDiagnostics atomic.Int32
	h := overlayServeHandler(t, p, func(i typesys.Index, a *typesys.BuildArtifacts) sema.Result {
		result := plainOverlayAnalysis(i, a)
		if calls.Add(1) == 1 {
			firstDiagnostics.Store(int32(len(result.Diagnostics)))
			close(started)
			<-release
		}
		return result
	})
	c := startServe(t, h)
	uri := uris["First"]
	c.send(openMessage(uri, initial))
	if n, ok := publishCount(t, c.mustNext(5*time.Second), uri); !ok || n != 0 {
		t.Fatalf("open publish = %d, %v", n, ok)
	}
	c.send(changeMessage(uri, 2, "public class First { void run() { Integer x = 'wrong'; } }"))
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("overlay analysis did not start")
	}
	if firstDiagnostics.Load() == 0 {
		t.Fatal("first text has no diagnostic; the test proves nothing")
	}
	c.send(changeMessage(uri, 3, "public class First { void run() { Integer x = 2; } }"))
	time.Sleep(50 * time.Millisecond)
	close(release)
	for {
		m, ok := c.next(10 * time.Second)
		if !ok {
			t.Fatal("no publish for the latest text")
		}
		n, mine := publishCount(t, m, uri)
		if !mine {
			continue
		}
		if n != 0 {
			t.Fatalf("published %d diagnostics from a superseded text", n)
		}
		break
	}
	for {
		m, ok := c.next(200 * time.Millisecond)
		if !ok {
			break
		}
		if n, mine := publishCount(t, m, uri); mine && n != 0 {
			t.Fatalf("late publish of %d stale diagnostics", n)
		}
	}
}

// Three quick edits while an analysis is held run at most two analyses: the
// middle generation is superseded before its turn and skipped.
func TestSupersededOverlayAnalysisIsSkipped(t *testing.T) {
	initial := "public class First { void run() { Integer x = 1; } }"
	p, uris := overlayServeProject(t, map[string]string{"First": initial})
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	h := overlayServeHandler(t, p, func(i typesys.Index, a *typesys.BuildArtifacts) sema.Result {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		return plainOverlayAnalysis(i, a)
	})
	c := startServe(t, h)
	uri := uris["First"]
	c.send(openMessage(uri, initial))
	c.mustNext(5 * time.Second)
	c.send(changeMessage(uri, 2, "public class First { void run() { Integer x = 2; } }"))
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("overlay analysis did not start")
	}
	c.send(changeMessage(uri, 3, "public class First { void run() { Integer x = 3; } }"))
	time.Sleep(150 * time.Millisecond)
	c.send(changeMessage(uri, 4, "public class First { void run() { Integer x = 'last'; } }"))
	time.Sleep(150 * time.Millisecond)
	close(release)
	for {
		m, ok := c.next(10 * time.Second)
		if !ok {
			t.Fatal("no publish for the last edit")
		}
		if n, mine := publishCount(t, m, uri); mine && n > 0 {
			break
		}
	}
	if got := calls.Load(); got > 2 {
		t.Fatalf("overlay analyses = %d, want at most 2", got)
	}
}

// Typing into two documents while a third is saved, under -race.
func TestOverlayTypingWhileSaving(t *testing.T) {
	sources := map[string]string{
		"First":  "public class First { void run() {} }",
		"Second": "public class Second { void run() {} }",
		"Third":  "public class Third { void run() {} }",
	}
	p, uris := overlayServeProject(t, sources)
	h := overlayServeHandler(t, p, plainOverlayAnalysis)
	h.SetRefresher(func(ctx context.Context, request RefreshRequest) (Base, error) {
		loaded, err := project.Load(p.Root)
		if err != nil {
			return Base{}, err
		}
		index, artifacts := typesys.BuildWithArtifacts(loaded, schema.Schema{})
		return Base{Index: index, Project: loaded, Artifacts: &artifacts}, nil
	})
	c := startServe(t, h)
	var drained sync.WaitGroup
	drained.Add(1)
	go func() {
		defer drained.Done()
		for range c.out {
		}
	}()
	for name, text := range sources {
		c.send(openMessage(uris[name], text))
	}
	third := pathFromURI(uris["Third"])
	for i := 0; i < 20; i++ {
		c.send(changeMessage(uris["First"], i+2, "public class First { void run() { Integer x = "+strconv.Itoa(i)+"; } }"))
		c.send(changeMessage(uris["Second"], i+2, "public class Second { void run() { Integer y = 'v"+strconv.Itoa(i)+"'; } }"))
		body := "public class Third { void run() { Integer z = " + strconv.Itoa(i) + "; } }"
		c.send(changeMessage(uris["Third"], i+2, body))
		if err := os.WriteFile(third, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		c.send(map[string]any{"method": "textDocument/didSave", "params": map[string]any{"textDocument": map[string]any{"uri": uris["Third"]}}})
		_ = h.DocumentSymbols(uris["First"])
		time.Sleep(5 * time.Millisecond)
	}
	c.send(map[string]any{"id": 99, "method": "shutdown"})
	if err := <-c.served; err != nil {
		t.Fatal(err)
	}
	drained.Wait()
}
