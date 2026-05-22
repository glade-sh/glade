package playground

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if !strings.Contains(rec.Body.String(), "OAER Apex Playground") || !strings.Contains(rec.Body.String(), "/playground/api/run") {
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

func TestServerLoadsProjectReferenceAsLocalSource(t *testing.T) {
	projectRoot := t.TempDir()
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"name":"local-ref","namespace":"verifiable","sourceApiVersion":"65.0"}`)
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
