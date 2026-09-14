package playground

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

func TestServerHealthAndReadinessRoutes(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test-version"})
	for _, path := range []string{"/healthz", "/readyz"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("GET %s = %d cache=%q body=%s", path, rec.Code, rec.Header().Get("Cache-Control"), rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "test-version") {
			t.Fatalf("GET %s omitted version: %s", path, rec.Body.String())
		}
	}

	handler.runner.initErr = errors.New("database unavailable")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"status":"unavailable"`) {
		t.Fatalf("unready response = %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "database unavailable") {
		t.Fatalf("readiness response exposed internal detail: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("liveness should remain available, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestServerReadinessDoesNotWaitForExecution(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test-version"})
	handler.runner.mu.Lock()
	defer handler.runner.mu.Unlock()

	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		close(done)
	}()
	select {
	case <-done:
		if rec.Code != http.StatusOK {
			t.Fatalf("readiness status = %d body=%s", rec.Code, rec.Body.String())
		}
	case <-time.After(time.Second):
		t.Fatal("readiness waited for the execution mutex")
	}
}

func TestPublicWorkspaceDescribesEnforcedPolicy(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test", Public: true})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("workspace status = %d body=%s", rec.Code, rec.Body.String())
	}
	var meta WorkspaceMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	policy := meta.Policy
	if !policy.Public || policy.RunMode != string(RunModeScratch) || policy.LimitMode != vm.LimitModeStrict {
		t.Fatalf("public policy modes = %#v", policy)
	}
	if policy.RunTimeoutMS != defaultPublicRunTimeout.Milliseconds() || policy.RatePerMinute != defaultPublicRatePerMinute {
		t.Fatalf("public policy request limits = %#v", policy)
	}
	if policy.MaxWorkspaceFiles != defaultPublicMaxWorkspaceFiles || policy.MaxWorkspaceBytes != defaultPublicMaxWorkspaceBytes {
		t.Fatalf("public workspace policy = %#v", policy)
	}
	if policy.LimitCaps != defaultPublicLimitCaps() {
		t.Fatalf("public governor caps = %#v", policy.LimitCaps)
	}
}

func TestPublicRunTimeoutReturnsActionableResponse(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test", Public: true, RunTimeout: time.Nanosecond})
	body, err := json.Marshal(RunRequest{AnonymousBody: "System.debug('timeout');"})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(body)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("timeout status = %d body=%s", rec.Code, rec.Body.String())
	}
	var result RunResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != RunStatusRuntimeError || !strings.Contains(result.ErrorMessage, "execution timed out after 1ns") ||
		!strings.Contains(result.ErrorMessage, "try a smaller example") {
		t.Fatalf("timeout result = %#v", result)
	}
}

func TestServerWorkspaceAndRunRoutes(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("workspace status = %d body=%s", rec.Code, rec.Body.String())
	}

	body, _ := json.Marshal(RunRequest{AnonymousBody: "System.debug('route');", Mode: RunModeScratch, LimitMode: "permissive", UseCache: true})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(body))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", rec.Code, rec.Body.String())
	}
	var result RunResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Status != RunStatusPass || len(result.Logs) != 1 || result.Logs[0] != "route" {
		t.Fatalf("result = %#v", result)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/playground/api/reset", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("reset status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/playground/api/runs/latest", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"found":false`) {
		t.Fatalf("latest after reset = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServerResetReportsResultCacheFailure(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test"})
	latestPath := filepath.Join(ws.DataRoot, "cache", "runs", "latest.json")
	if err := os.MkdirAll(latestPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(latestPath, "keep"), []byte("sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/playground/api/reset", nil))
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "could not clear the previous run result") {
		t.Fatalf("reset cache failure = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServeWorkspaceSymlinkReturnsNotFound(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.cls")
	if err := os.WriteFile(outside, []byte("outside sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(ws.Root, "force-app/main/default/classes/Outside.cls")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/force-app/main/default/classes/Outside.cls", nil))
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "outside sentinel") {
		t.Fatalf("GET symlink = %d %q", rec.Code, rec.Body.String())
	}
}

func TestSeedSymlinkDoesNotReadOutsideFile(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "seed.json")
	if err := os.WriteFile(outside, []byte(`{"records":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(ws.Root, "seed.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(ws.Root, "seed.json")); err != nil {
		t.Fatal(err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/playground/api/seed", nil))
	if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "outside sentinel") {
		t.Fatalf("seed symlink = %d %q", rec.Code, rec.Body.String())
	}
}

func TestServerWorkspaceIncludesConfiguredDBPath(t *testing.T) {
	dataRoot := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "org.sqlite")
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test", DBPath: dbPath})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("workspace status = %d body=%s", rec.Code, rec.Body.String())
	}
	var meta WorkspaceMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("decode workspace: %v", err)
	}
	if meta.DBPath != dbPath {
		t.Fatalf("db path = %q, want %q", meta.DBPath, dbPath)
	}
}

func TestPublicServerWorkspaceOmitsConfiguredDBPath(t *testing.T) {
	dataRoot := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "org.sqlite")
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test", DBPath: dbPath, Public: true})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("workspace status = %d body=%s", rec.Code, rec.Body.String())
	}
	var meta WorkspaceMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("decode workspace: %v", err)
	}
	if meta.DBPath != "" {
		t.Fatalf("public db path = %q, want empty", meta.DBPath)
	}
}

func TestServerDatabaseRouteShowsLatestScratchRunRows(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test"})

	body, _ := json.Marshal(RunRequest{
		AnonymousBody: "Account account = new Account(Name = 'Browse Me'); insert account;",
		Mode:          RunModeScratch,
		LimitMode:     "permissive",
		UseCache:      false,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(body))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/playground/api/database", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("database status = %d body=%s", rec.Code, rec.Body.String())
	}
	var snapshot DatabaseSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	account := databaseObjectByName(snapshot, "Account")
	if account == nil {
		t.Fatalf("Account object missing: %#v", snapshot.Objects)
	}
	if account.RecordCount != 1 || len(account.Rows) != 1 {
		t.Fatalf("Account rows = %d %#v", account.RecordCount, account.Rows)
	}
	if got := account.Rows[0].Fields["Name"]; got != "Browse Me" {
		t.Fatalf("Account.Name = %#v", got)
	}
}

func TestServerDatabaseRouteReexecutesWhenCachedResultHasNoOrgSnapshot(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test"})
	runBody, _ := json.Marshal(RunRequest{
		AnonymousBody: "Account account = new Account(Name = 'After Reset'); insert account;",
		Mode:          RunModeScratch,
		LimitMode:     "permissive",
		UseCache:      true,
	})

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(runBody))
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("run %d status = %d body=%s", i, rec.Code, rec.Body.String())
		}
		if i == 0 {
			reset := httptest.NewRecorder()
			resetReq := httptest.NewRequest(http.MethodPost, "/playground/api/reset", nil)
			handler.ServeHTTP(reset, resetReq)
			if reset.Code != http.StatusOK {
				t.Fatalf("reset status = %d body=%s", reset.Code, reset.Body.String())
			}
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/playground/api/database", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("database status = %d body=%s", rec.Code, rec.Body.String())
	}
	var snapshot DatabaseSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	account := databaseObjectByName(snapshot, "Account")
	if account == nil || account.RecordCount != 1 {
		t.Fatalf("Account rows after cached rerun = %#v", account)
	}
}

func databaseObjectByName(snapshot DatabaseSnapshot, name string) *DatabaseObject {
	for i := range snapshot.Objects {
		if snapshot.Objects[i].Name == name {
			return &snapshot.Objects[i]
		}
	}
	return nil
}

func TestWorkspaceMetadataIgnoresDotFilesAndDirectories(t *testing.T) {
	dataRoot := t.TempDir()
	root := filepath.Join(dataRoot, "workspaces", "default")
	writePlaygroundTestFile(t, filepath.Join(root, ".claude/settings.json"), "{}")
	writePlaygroundTestFile(t, filepath.Join(root, ".claude/worktrees/tmp/force-app/main/default/classes/HiddenProbe.cls"), "public class HiddenProbe {}")

	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	meta, err := ws.Metadata()
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}
	for _, file := range meta.Files {
		if strings.HasPrefix(file.Path, ".") || strings.Contains(file.Path, "/.") {
			t.Fatalf("dot path listed in workspace metadata: %#v", file)
		}
	}
	if len(meta.Files) == 0 {
		t.Fatalf("metadata files = %#v, want default scratch files", meta.Files)
	}
	if meta.AnonymousBody == "" {
		t.Fatalf("anonymous body was not initialized")
	}
}

func TestServerServesEmbeddedPlaygroundUI(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/playground/", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ui status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Glade Playground") || !strings.Contains(rec.Body.String(), "/playground/api/run") {
		t.Fatalf("ui body = %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `id="root"`) || !strings.Contains(rec.Body.String(), `/playground/assets/app.js`) {
		t.Fatalf("ui is not the React playground shell: %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/playground/assets/app.js", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("asset status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "createRoot") {
		t.Fatalf("asset body did not look like the React bundle")
	}
}

func TestServerFileSaveRejectsStaleVersion(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test"})

	save := FileSaveRequest{Path: "force-app/main/default/classes/Extra.cls", Content: "public class Extra {}", Version: 0}
	body, _ := json.Marshal(save)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/playground/api/files", bytes.NewReader(body))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first save status = %d body=%s", rec.Code, rec.Body.String())
	}

	save.Content = "public class Extra { }"
	body, _ = json.Marshal(save)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/playground/api/files", bytes.NewReader(body))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale save status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServerSaveSourceInvalidatesProjectRuntime(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test"})
	handler.runner.runtimeTemplate = &cachedRuntimeTemplate{workspaceHash: "warm"}
	handler.runner.lastOrgCacheKey = "warm-result"

	meta, err := ws.Metadata()
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}
	versions := make(map[string]int)
	for _, file := range meta.Files {
		versions[file.Path] = file.Version
	}
	save := FileSaveRequest{
		Path:    "force-app/main/default/classes/AccountPlayground.cls",
		Content: "public class AccountPlayground { public static String marker(){ return 'fresh'; } }",
		Version: versions["force-app/main/default/classes/AccountPlayground.cls"],
	}
	body, _ := json.Marshal(save)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/playground/api/files", bytes.NewReader(body))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save status = %d body=%s", rec.Code, rec.Body.String())
	}
	if handler.runner.runtimeTemplate != nil {
		t.Fatalf("runtime template was not invalidated")
	}
	if handler.runner.lastOrgCacheKey != "" {
		t.Fatalf("last org cache key = %q, want empty", handler.runner.lastOrgCacheKey)
	}
}

func TestServerDeleteSourceInvalidatesProjectRuntime(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	if _, err := ws.SaveFile(FileSaveRequest{
		Path:    "force-app/main/default/classes/DeleteMe.cls",
		Content: "public class DeleteMe {}",
		Version: 0,
	}); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test"})
	handler.runner.runtimeTemplate = &cachedRuntimeTemplate{workspaceHash: "warm"}
	handler.runner.lastOrgCacheKey = "warm-result"

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/playground/api/files?path=force-app/main/default/classes/DeleteMe.cls", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}
	if handler.runner.runtimeTemplate != nil {
		t.Fatalf("runtime template was not invalidated")
	}
	if handler.runner.lastOrgCacheKey != "" {
		t.Fatalf("last org cache key = %q, want empty", handler.runner.lastOrgCacheKey)
	}
}

func TestServerLoadExampleInvalidatesProjectRuntime(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test", ShowExamples: true})
	handler.runner.runtimeTemplate = &cachedRuntimeTemplate{workspaceHash: "warm"}
	handler.runner.lastOrgCacheKey = "warm-result"

	body, _ := json.Marshal(map[string]string{"id": "trigger-contact-task"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/playground/api/examples/load", bytes.NewReader(body))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("load example status = %d body=%s", rec.Code, rec.Body.String())
	}
	if handler.runner.runtimeTemplate != nil {
		t.Fatalf("runtime template was not invalidated")
	}
	if handler.runner.lastOrgCacheKey != "" {
		t.Fatalf("last org cache key = %q, want empty", handler.runner.lastOrgCacheKey)
	}
}

func TestServerListsLoadsAndRunsExampleProject(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test", ShowExamples: true})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/playground/api/examples", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("examples status = %d body=%s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Examples []ExampleProject `json:"examples"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode examples: %v", err)
	}
	if len(listed.Examples) < 3 {
		t.Fatalf("examples = %#v", listed.Examples)
	}

	body, _ := json.Marshal(map[string]string{"id": "trigger-contact-task"})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/playground/api/examples/load", bytes.NewReader(body))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("load example status = %d body=%s", rec.Code, rec.Body.String())
	}
	var meta WorkspaceMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("decode workspace: %v", err)
	}
	if meta.ExampleID != "trigger-contact-task" {
		t.Fatalf("example id = %q", meta.ExampleID)
	}
	if !strings.Contains(meta.AnonymousBody, "TriggerExample.run") {
		t.Fatalf("anonymous body = %q", meta.AnonymousBody)
	}
	foundTrigger := false
	for _, file := range meta.Files {
		if file.Path == "force-app/main/default/triggers/AccountTaskTrigger.trigger" {
			foundTrigger = true
			break
		}
	}
	if !foundTrigger {
		t.Fatalf("files = %#v", meta.Files)
	}

	body, _ = json.Marshal(RunRequest{AnonymousBody: meta.AnonymousBody, Mode: RunModeScratch, LimitMode: "permissive", UseCache: false})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(body))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", rec.Code, rec.Body.String())
	}
	var result RunResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Status != RunStatusPass || len(result.Logs) == 0 {
		t.Fatalf("result = %#v", result)
	}
}

func TestServerHidesBuiltInExamplesUnlessEnabled(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/playground/api/examples", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("examples status = %d body=%s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Examples []ExampleProject `json:"examples"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode examples: %v", err)
	}
	if !strings.Contains(rec.Body.String(), `"examples":[]`) {
		t.Fatalf("examples should be an empty array, body=%s", rec.Body.String())
	}
	if len(listed.Examples) != 0 {
		t.Fatalf("examples = %#v", listed.Examples)
	}
}

func TestServerListsLoadsAndRunsLocalProjectReference(t *testing.T) {
	projectRoot := t.TempDir()
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"name":"local-ref","namespace":"","sourceApiVersion":"65.0"}`)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "force-app/main/default/classes/LocalProbe.cls"), `public class LocalProbe {
  public static String run() {
    return 'local-ref-loaded';
  }
}

`)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "anonymous.apex"), `System.debug(LocalProbe.run());
`)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "packages/app/force-app/main/default/classes/NestedProbe.cls"), `public class NestedProbe {}
`)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, ".claude/worktrees/tmp/force-app/main/default/classes/HiddenProbe.cls"), `public class HiddenProbe {}
`)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "force-app/main/default/classes/.DotProbe.cls"), `public class DotProbe {}
`)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, ".scratch.json"), `{"not":"data"}`)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "config/settings.json"), `{"not":"seed"}`)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "force-app/main/default/classes/Oversize.cls"), strings.Repeat("x", maxPlaygroundFileSize+1))

	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{
		Version: "test",
		ProjectReferences: []ProjectReference{{
			Name: "Local Probe",
			Path: projectRoot,
		}},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/playground/api/examples", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("examples status = %d body=%s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Examples []ExampleProject `json:"examples"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode examples: %v", err)
	}
	refID := ""
	for _, example := range listed.Examples {
		if example.Source != "local" {
			t.Fatalf("project ref list included built-in example: %#v", example)
		}
		if example.Source == "local" && example.Name == "Local Probe" {
			refID = example.ID
			if example.FileCount != 0 {
				t.Fatalf("local ref file count = %d", example.FileCount)
			}
			break
		}
	}
	if refID == "" {
		t.Fatalf("local reference not listed: %#v", listed.Examples)
	}

	body, _ := json.Marshal(map[string]string{"id": refID})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/playground/api/examples/load", bytes.NewReader(body))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "exceeds") {
		t.Fatalf("oversize load status = %d body=%s", rec.Code, rec.Body.String())
	}

	if err := os.Remove(filepath.Join(projectRoot, "force-app/main/default/classes/Oversize.cls")); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/playground/api/examples/load", bytes.NewReader(body))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("load local ref status = %d body=%s", rec.Code, rec.Body.String())
	}
	var meta WorkspaceMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("decode workspace: %v", err)
	}
	if meta.ExampleID != refID {
		t.Fatalf("example id = %q, want %q", meta.ExampleID, refID)
	}
	if !strings.Contains(meta.AnonymousBody, "LocalProbe.run") {
		t.Fatalf("anonymous body = %q", meta.AnonymousBody)
	}
	foundNested := false
	for _, file := range meta.Files {
		if file.Path == "packages/app/force-app/main/default/classes/NestedProbe.cls" {
			foundNested = true
			break
		}
	}
	if !foundNested {
		t.Fatalf("nested folder path not retained: %#v", meta.Files)
	}
	for _, file := range meta.Files {
		if strings.HasPrefix(file.Path, ".") || strings.Contains(file.Path, "/.") {
			t.Fatalf("dot path loaded from project reference: %#v", file)
		}
		if file.Path == "config/settings.json" && file.Kind == "data" {
			t.Fatalf("non-seed json classified as data: %#v", file)
		}
	}

	body, _ = json.Marshal(RunRequest{AnonymousBody: meta.AnonymousBody, Mode: RunModeScratch, LimitMode: "permissive", UseCache: false})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(body))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", rec.Code, rec.Body.String())
	}
	var result RunResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Status != RunStatusPass || len(result.Logs) != 1 || result.Logs[0] != "local-ref-loaded" {
		t.Fatalf("result = %#v", result)
	}

	reopened, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("reopen OpenWorkspace() error = %v", err)
	}
	handler = NewServer(reopened, ServerOptions{
		Version: "test",
		ProjectReferences: []ProjectReference{{
			Name: "Local Probe",
			Path: projectRoot,
		}},
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reopened workspace status = %d body=%s", rec.Code, rec.Body.String())
	}
	meta = WorkspaceMetadata{}
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("decode reopened workspace: %v", err)
	}
	if meta.ExampleID != "" {
		t.Fatalf("reopened example id = %q, want empty without expensive project-ref match", meta.ExampleID)
	}
}

func TestPublicProjectReferenceRespectsWorkspaceLimits(t *testing.T) {
	projectRoot := t.TempDir()
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), sfdxProjectJSON)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "force-app/main/default/classes/One.cls"), "public class One {}")
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "force-app/main/default/classes/Two.cls"), "public class Two {}")
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test", Public: true, MaxWorkspaceFiles: 2, MaxWorkspaceBytes: 1024, ProjectReferences: []ProjectReference{{Name: "Local", Path: projectRoot}}})
	before, err := ws.Metadata()
	if err != nil {
		t.Fatal(err)
	}
	request, _ := json.Marshal(map[string]string{"id": handler.projectRefs[0].ID})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/playground/api/examples/load", bytes.NewReader(request)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("count-limited load = %d %s", rec.Code, rec.Body.String())
	}
	after, err := ws.Metadata()
	if err != nil {
		t.Fatal(err)
	}
	if after.WorkspaceHash != before.WorkspaceHash {
		t.Fatal("workspace changed after rejected project reference")
	}
	handler = NewServer(ws, ServerOptions{Version: "test", Public: true, MaxWorkspaceFiles: 16, MaxWorkspaceBytes: 10, ProjectReferences: []ProjectReference{{Name: "Local", Path: projectRoot}}})
	request, _ = json.Marshal(map[string]string{"id": handler.projectRefs[0].ID})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/playground/api/examples/load", bytes.NewReader(request)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("byte-limited load = %d %s", rec.Code, rec.Body.String())
	}
	handler = NewServer(ws, ServerOptions{Version: "test", Public: true, MaxWorkspaceFiles: 5, MaxWorkspaceBytes: 1024, ProjectReferences: []ProjectReference{{Name: "Local", Path: projectRoot}}})
	request, _ = json.Marshal(map[string]string{"id": handler.projectRefs[0].ID})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/playground/api/examples/load", bytes.NewReader(request)))
	if rec.Code != http.StatusOK {
		t.Fatalf("bounded load = %d %s", rec.Code, rec.Body.String())
	}
	meta, err := ws.Metadata()
	if err != nil {
		t.Fatal(err)
	}
	if file := workspaceFileByPath(meta.Files, "force-app/main/default/classes/One.cls"); file == nil || !file.ReadOnly {
		t.Fatalf("bounded project file = %#v", file)
	}
}

func TestServerLoadsProjectReferenceAsLocalSource(t *testing.T) {
	projectRoot := t.TempDir()
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"name":"local-ref","namespace":"samplepkg","sourceApiVersion":"65.0"}`)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "force-app/main/default/classes/NextGenSettingService.cls"), `public class NextGenSettingService {
  public static String activateNextGenSetting() {
    return 'activated';
  }
}
`)
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "anonymous.apex"), `System.debug(NextGenSettingService.activateNextGenSetting());
`)

	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{
		Version: "test",
		ProjectReferences: []ProjectReference{{
			Name: "Namespaced Local Source",
			Path: projectRoot,
		}},
	})

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/playground/api/examples", nil)
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("examples status = %d body=%s", listRec.Code, listRec.Body.String())
	}
	var listed struct {
		Examples []ExampleProject `json:"examples"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode examples: %v", err)
	}
	if len(listed.Examples) != 1 {
		t.Fatalf("examples = %#v", listed.Examples)
	}

	loadBody, _ := json.Marshal(map[string]string{"id": listed.Examples[0].ID})
	loadRec := httptest.NewRecorder()
	loadReq := httptest.NewRequest(http.MethodPost, "/playground/api/examples/load", bytes.NewReader(loadBody))
	handler.ServeHTTP(loadRec, loadReq)
	if loadRec.Code != http.StatusOK {
		t.Fatalf("load local ref status = %d body=%s", loadRec.Code, loadRec.Body.String())
	}
	var meta WorkspaceMetadata
	if err := json.Unmarshal(loadRec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("decode workspace: %v", err)
	}

	runBody, _ := json.Marshal(RunRequest{AnonymousBody: meta.AnonymousBody, Mode: RunModeScratch, LimitMode: "permissive", UseCache: false})
	runRec := httptest.NewRecorder()
	runReq := httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(runBody))
	handler.ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", runRec.Code, runRec.Body.String())
	}
	var result RunResult
	if err := json.Unmarshal(runRec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Status != RunStatusPass || len(result.Logs) != 1 || result.Logs[0] != "activated" {
		t.Fatalf("result = %#v", result)
	}
}

func writePlaygroundTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPublicServerRateLimitsMutatingEndpointsByForwardedIP(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{Version: "test", Public: true, RatePerMinute: 1})

	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/playground/api/reset", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
		handler.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("reset %d status = %d body=%s, want %d", i, rec.Code, rec.Body.String(), want)
		}
		if want == http.StatusTooManyRequests && rec.Header().Get("Retry-After") != "60" {
			t.Fatalf("rate limit Retry-After = %q, want 60", rec.Header().Get("Retry-After"))
		}
	}
}

func TestPublicServerForcesScratchStrictRun(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	handler := NewServer(ws, ServerOptions{
		Version:    "test",
		Public:     true,
		RunTimeout: 30 * time.Second,
	})

	body, _ := json.Marshal(RunRequest{
		AnonymousBody: "Account account = new Account(Name = 'No Persist'); insert account;",
		Mode:          RunModePersist,
		LimitMode:     "permissive",
		UseCache:      true,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(body))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", rec.Code, rec.Body.String())
	}
	var result RunResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.LimitMode != "strict" {
		t.Fatalf("limit mode = %q, want strict", result.LimitMode)
	}
	org := handler.runner.Org()
	if account := org.Objects["Account"]; len(account.Records) != 0 {
		t.Fatalf("public persist wrote %d account records to shared org", len(account.Records))
	}
	if _, found, err := handler.runner.cache.Latest(); err != nil || found {
		t.Fatalf("public run cached latest result: found=%v err=%v", found, err)
	}
	cacheEntries, err := os.ReadDir(filepath.Join(dataRoot, "cache", "cache"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if len(cacheEntries) != 0 {
		t.Fatalf("public run wrote %d per-key cache files", len(cacheEntries))
	}
}
