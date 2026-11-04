package playground

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

type case339DeadlineLifecycleWriter struct {
	*httptest.ResponseRecorder
	deadline time.Time
}

func (w *case339DeadlineLifecycleWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}
func (w *case339DeadlineLifecycleWriter) SetReadDeadline(time.Time) error { return nil }

type case339DeadlineLifecycleBody struct {
	writer            *case339DeadlineLifecycleWriter
	readUnderDeadline bool
	io.Reader
}

func (b *case339DeadlineLifecycleBody) Read(p []byte) (int, error) {
	if !b.writer.deadline.IsZero() {
		b.readUnderDeadline = true
	}
	return b.Reader.Read(p)
}
func (*case339DeadlineLifecycleBody) Close() error { return nil }

func TestCase339ReviewResponseDeadlineLifecycle(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true})
	defer s.Close()
	for _, path := range []string{"/playground/api/reset", "/playground/api/run"} {
		t.Run(path, func(t *testing.T) {
			w := &case339DeadlineLifecycleWriter{ResponseRecorder: httptest.NewRecorder()}
			b := &case339DeadlineLifecycleBody{writer: w, Reader: strings.NewReader(`{}`)}
			r := httptest.NewRequest(http.MethodPost, path, nil)
			r.Body = b
			r.ContentLength = 2
			s.ServeHTTP(w, r)
			if b.readUnderDeadline {
				t.Error("response deadline was running while reading request")
			}
			if w.deadline.IsZero() {
				t.Error("response deadline cleared before net/http flush")
			}
		})
	}
	for _, path := range []string{"/playground/", "/playground/api/workspace", "/playground/api/runs/latest"} {
		w := &case339DeadlineLifecycleWriter{ResponseRecorder: httptest.NewRecorder()}
		s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.deadline.IsZero() {
			t.Errorf("%s: response deadline cleared before flush", path)
		}
	}
}

func TestCase339ReviewCacheSeparatesOutputLimitAndCaps(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test"})
	req := RunRequest{AnonymousBody: "System.debug('" + strings.Repeat("x", 128) + "');", UseCache: true}
	first, err := runner.Run(t.Context(), req)
	if err != nil || first.Status != RunStatusPass {
		t.Fatalf("unlimited run: %#v %v", first, err)
	}
	req.OutputLimit = 16
	small, err := runner.Run(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if small.CacheHit || small.CacheKey == first.CacheKey || small.Status == RunStatusPass {
		t.Fatalf("limited run reused unlimited output: %#v", small)
	}
	req.OutputLimit = 2048
	large, err := runner.Run(t.Context(), req)
	if err != nil || large.Status != RunStatusPass || large.CacheHit || large.CacheKey == first.CacheKey {
		t.Fatalf("larger run reused a different limit: %#v %v", large, err)
	}
	warm, err := runner.Run(t.Context(), req)
	if err != nil || !warm.CacheHit {
		t.Fatalf("same-limit warm run: %#v %v", warm, err)
	}
	req.LimitCaps = &vm.LimitCaps{Queries: 1}
	a, err := runner.Run(t.Context(), req)
	if err != nil || a.CacheHit || a.CacheKey == large.CacheKey {
		t.Fatalf("custom caps reused defaults: %#v %v", a, err)
	}
	req.LimitCaps = &vm.LimitCaps{Queries: 2}
	b, err := runner.Run(t.Context(), req)
	if err != nil || b.CacheHit || b.CacheKey == a.CacheKey {
		t.Fatalf("custom caps reused different caps: %#v %v", b, err)
	}
}

func TestCase339ReviewCacheSeparatesExplicitCaps(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test"})
	req := RunRequest{AnonymousBody: "Integer x = 1;", UseCache: true, LimitCaps: &vm.LimitCaps{Queries: 1}}
	first, err := runner.Run(t.Context(), req)
	if err != nil || first.Status != RunStatusPass {
		t.Fatalf("first: %#v %v", first, err)
	}
	req.LimitCaps = &vm.LimitCaps{Queries: 2}
	second, err := runner.Run(t.Context(), req)
	if err != nil || second.CacheHit || first.CacheKey == second.CacheKey {
		t.Fatalf("different caps reused cache: %#v %v", second, err)
	}
	warm, err := runner.Run(t.Context(), req)
	if err != nil || !warm.CacheHit {
		t.Fatalf("same caps did not reuse cache: %#v %v", warm, err)
	}
}
