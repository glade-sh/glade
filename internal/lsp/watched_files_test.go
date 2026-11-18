package lsp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

type recordingRefresher struct {
	mu       sync.Mutex
	requests []RefreshRequest
	root     string
}

func (r *recordingRefresher) refresh(_ context.Context, request RefreshRequest) (Base, error) {
	r.mu.Lock()
	r.requests = append(r.requests, request)
	r.mu.Unlock()
	p, err := project.Load(r.root)
	if err != nil {
		return Base{}, err
	}
	index, artifacts := typesys.BuildWithArtifacts(p, schema.Schema{})
	return Base{Index: index, Project: p, Artifacts: &artifacts}, nil
}

func (r *recordingRefresher) snapshot() []RefreshRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]RefreshRequest(nil), r.requests...)
}

func (r *recordingRefresher) waitFor(t *testing.T, n int) []RefreshRequest {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if got := r.snapshot(); len(got) >= n {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("refreshes = %d, want %d", len(r.snapshot()), n)
	return nil
}

func watchedServe(t *testing.T, sources map[string]string) (*Handler, *recordingRefresher, map[string]DocumentURI, *serveClient) {
	t.Helper()
	p, uris := overlayServeProject(t, sources)
	h := overlayServeHandler(t, p, plainOverlayAnalysis)
	r := &recordingRefresher{root: p.Root}
	h.SetRefresher(r.refresh)
	return h, r, uris, startServe(t, h)
}

func watchedMessage(events ...FileEvent) map[string]any {
	return map[string]any{"method": "workspace/didChangeWatchedFiles", "params": DidChangeWatchedFilesParams{Changes: events}}
}

func TestWatchedFilesRegistrationNeedsCapability(t *testing.T) {
	for _, supported := range []bool{false, true} {
		t.Run(strconv.FormatBool(supported), func(t *testing.T) {
			c := startServe(t, NewHandler(sampleIndex(t)))
			caps := map[string]any{}
			if supported {
				caps = map[string]any{"workspace": map[string]any{"didChangeWatchedFiles": map[string]any{"dynamicRegistration": true}}}
			}
			c.send(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"capabilities": caps}})
			c.mustNext(5 * time.Second)
			c.send(map[string]any{"method": "initialized", "params": map[string]any{}})
			m, ok := c.next(300 * time.Millisecond)
			if !supported {
				if ok {
					t.Fatalf("registration sent without the capability: %v", m)
				}
				return
			}
			if !ok {
				t.Fatal("no registration request")
			}
			var method, id string
			_ = json.Unmarshal(m["method"], &method)
			_ = json.Unmarshal(m["id"], &id)
			if method != "client/registerCapability" || id != watchRegistrationID {
				t.Fatalf("registration = %v", m)
			}
			// The client's response to our request gets no reply.
			c.send(map[string]any{"id": watchRegistrationID, "result": nil})
			if m, ok := c.next(300 * time.Millisecond); ok {
				t.Fatalf("answered a client response: %v", m)
			}
		})
	}
}

func TestWatchedFilesDropSaveEchoAndRefreshExternalChange(t *testing.T) {
	_, r, uris, c := watchedServe(t, map[string]string{"First": "public class First {}"})
	path := pathFromURI(uris["First"])
	c.send(watchedMessage(FileEvent{URI: uris["First"], Type: fileChanged}, FileEvent{URI: uriFromPath(path + "-meta.xml"), Type: fileChanged}))
	time.Sleep(watchQuiet + 300*time.Millisecond)
	if got := r.snapshot(); len(got) != 0 {
		t.Fatalf("unchanged file refreshed: %#v", got)
	}
	if err := os.WriteFile(path, []byte("public class First { void added() {} }"), 0644); err != nil {
		t.Fatal(err)
	}
	c.send(watchedMessage(FileEvent{URI: uris["First"], Type: fileChanged}))
	got := r.waitFor(t, 1)
	if len(got[0].Changed) != 1 || got[0].Changed[0] != path || len(got[0].Created) != 0 || len(got[0].Deleted) != 0 {
		t.Fatalf("request = %#v", got[0])
	}
	// Ignored directories never refresh.
	c.send(watchedMessage(FileEvent{URI: uriFromPath(filepath.Join(filepath.Dir(path), ".glade", "x.cls")), Type: fileCreated}))
	time.Sleep(watchQuiet + 300*time.Millisecond)
	if got := r.snapshot(); len(got) != 1 {
		t.Fatalf("ignored path refreshed: %#v", got)
	}
}

func TestWatchedCreateAndDeleteAreClassified(t *testing.T) {
	_, r, uris, c := watchedServe(t, map[string]string{"First": "public class First {}", "Second": "public class Second {}"})
	second := pathFromURI(uris["Second"])
	created := filepath.Join(filepath.Dir(second), "Third.cls")
	if err := os.WriteFile(created, []byte("public class Third {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(second); err != nil {
		t.Fatal(err)
	}
	c.send(watchedMessage(FileEvent{URI: uriFromPath(created), Type: fileCreated}, FileEvent{URI: uris["Second"], Type: fileDeleted}))
	got := r.waitFor(t, 1)
	if len(got) != 1 || len(got[0].Created) != 1 || got[0].Created[0] != created || len(got[0].Deleted) != 1 || got[0].Deleted[0] != second {
		t.Fatalf("requests = %#v", got)
	}
}

func TestWatchedBurstYieldsOneRefresh(t *testing.T) {
	_, r, uris, c := watchedServe(t, map[string]string{"First": "public class First {}"})
	dir := filepath.Dir(pathFromURI(uris["First"]))
	for i := 0; i < 200; i++ {
		c.send(watchedMessage(FileEvent{URI: uriFromPath(filepath.Join(dir, "Burst"+strconv.Itoa(i)+".cls")), Type: fileCreated}))
	}
	got := r.waitFor(t, 1)
	time.Sleep(watchQuiet + 300*time.Millisecond)
	if got = r.snapshot(); len(got) != 1 || len(got[0].Created) != 200 {
		t.Fatalf("burst refreshes = %d", len(got))
	}
}

func TestExternalChangeUnderDirtyBufferKeepsOverlay(t *testing.T) {
	h, r, uris, c := watchedServe(t, map[string]string{"First": "public class First {}"})
	uri := uris["First"]
	path := pathFromURI(uri)
	c.send(openMessage(uri, "public class First {}"))
	buffer := "public class First { void buffered() {} }"
	c.send(changeMessage(uri, 2, buffer))
	if err := os.WriteFile(path, []byte("public class First { void external() {} }"), 0644); err != nil {
		t.Fatal(err)
	}
	c.send(watchedMessage(FileEvent{URI: uri, Type: fileChanged}))
	r.waitFor(t, 1)
	doc := openDocument{Path: path, Text: buffer}
	doc.Digest = sha256.Sum256([]byte(buffer))
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		g := h.gen.Load()
		if g.overlay != nil && g.baseSeq == h.baseGen.Load().seq && reflects(g, doc) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("dirty buffer lost its overlay after an external change")
}
