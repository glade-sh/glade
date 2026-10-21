package playground

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPublicServerAsyncExplicitCapsContracts(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	writePlaygroundTestFile(t, filepath.Join(ws.Root, "force-app/main/default/classes/CapWorker.cls"), `
public class CapWorker implements Queueable {
  public void execute(QueueableContext context) {
    System.assertEquals(100, Limits.getLimitQueries());
    System.assertEquals(10000, Limits.getLimitCpuTime());
    System.debug('explicit caps checked');
  }
}
`)
	caps, ok := vm.LimitCapsForProfile("strict-sync")
	if !ok {
		t.Fatal("strict-sync profile missing")
	}
	// The request budget includes compilation and is independent of the Apex CPU cap.
	handler := NewServer(ws, ServerOptions{Version: "test", Public: true, PublicLimitCaps: caps, RunTimeout: 30 * time.Second})
	body, err := json.Marshal(RunRequest{AnonymousBody: "System.enqueueJob(new CapWorker());", Mode: RunModeScratch})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/playground/api/run", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", rec.Code, rec.Body.String())
	}
	var result RunResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != RunStatusPass || len(result.Logs) != 1 || result.Logs[0] != "explicit caps checked" {
		t.Fatalf("result = %#v", result)
	}
}
