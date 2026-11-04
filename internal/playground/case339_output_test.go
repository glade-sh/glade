package playground

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

func TestCase339PublicOutputIsBounded(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RunTimeout: 30 * time.Second})
	t.Cleanup(s.Close)
	source := "String msg = '" + strings.Repeat("x", 1024) + "'; for (Integer i = 0; i < 300; i++) System.debug(msg);"
	body, err := json.Marshal(RunRequest{AnonymousBody: source})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(body)))
	var result RunResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v: %s", err, rec.Body.String())
	}
	if result.Status == RunStatusCompileError || (len(result.Logs) == 0 && !strings.Contains(result.ErrorMessage, "output limit")) {
		t.Fatalf("program did not exercise debug output: %#v", result)
	}
	if rec.Body.Len() > 128*1024 {
		t.Fatalf("public response has %d bytes; want bounded output", rec.Body.Len())
	}
}

func TestCase339PublicRunRetainsBoundedTrace(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RunTimeout: 30 * time.Second})
	t.Cleanup(s.Close)
	body, err := json.Marshal(RunRequest{AnonymousBody: "Integer n = 1; System.debug(n);"})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(body)))
	var result RunResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != RunStatusPass || len(result.Trace) == 0 {
		t.Fatalf("public run lost its trace: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCase339PublicTraceTruncatesAtBudget(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RunTimeout: 30 * time.Second})
	t.Cleanup(s.Close)
	body, err := json.Marshal(RunRequest{AnonymousBody: "Integer n = 0; for (Integer i = 0; i < 1000; i++) { n++; }"})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(body)))
	var result RunResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != RunStatusPass {
		t.Fatalf("run failed: %d %s", rec.Code, rec.Body.String())
	}
	found := false
	for _, event := range result.Trace {
		if event.Name == "apex.trace.truncated" {
			found = true
		}
	}
	if !found || rec.Body.Len() > 128*1024 {
		t.Fatalf("trace truncation missing or response too large: events=%d bytes=%d", len(result.Trace), rec.Body.Len())
	}
}

type case339DeadlineWriter struct {
	*httptest.ResponseRecorder
	sawDeadline bool
}

func (w *case339DeadlineWriter) SetWriteDeadline(deadline time.Time) error {
	if !deadline.IsZero() {
		w.sawDeadline = true
	}
	return nil
}

func TestCase339PublicOutputSetsWriteDeadline(t *testing.T) {
	w := &case339DeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	writePublicRunResult(w, http.StatusOK, RunResult{Status: RunStatusPass})
	if !w.sawDeadline {
		t.Fatal("public output has no write deadline")
	}
}

func TestCase339PublicOutputRejectsAliasedValuesBeforeEncoding(t *testing.T) {
	text := strings.Repeat("x", 32*1024)
	result := RunResult{Status: RunStatusPass}
	for i := 0; i < 128; i++ {
		result.Vars = append(result.Vars, VarResult{Name: "v", Value: vm.Value{Kind: vm.ValueString, Text: text}})
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	w := httptest.NewRecorder()
	writePublicRunResult(case339ResponseWriter{w}, http.StatusOK, result)
	runtime.ReadMemStats(&after)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want output rejection", w.Code)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 2*1024*1024 {
		t.Fatalf("oversized aliased result allocated %d bytes while encoding", allocated)
	}
}
