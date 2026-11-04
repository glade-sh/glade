package playground

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestRunnerRetriesWhenProjectConfigChangesDuringIndexing(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test"})
	loads := 0
	runner.loadWorkspaceIndex = func(root string) (typesys.Index, []diagnostic.Diagnostic, error) {
		index, diagnostics, err := loadWorkspaceIndex(root)
		loads++
		if loads == 1 {
			manifest := `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`
			if err := os.WriteFile(filepath.Join(ws.Root, "sfdx-project.json"), []byte(manifest), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return index, diagnostics, err
	}
	result, err := runner.Run(t.Context(), RunRequest{AnonymousBody: "System.debug('ok');"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RunStatusPass || runner.runtimeTemplate == nil {
		t.Fatalf("status=%s, runtime present=%v; want a prepared runtime", result.Status, runner.runtimeTemplate != nil)
	}
	if got := runner.runtimeTemplate.apiVersion; got != "67.0" {
		t.Fatalf("execution API version = %q, want 67.0", got)
	}
	if loads < 2 {
		t.Fatalf("loads = %d; want a retry after the config change", loads)
	}
}
