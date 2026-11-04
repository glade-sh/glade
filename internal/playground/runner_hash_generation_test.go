package playground

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestRunnerRetriesWhenSourceChangesDuringIndexing(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(ws.Root, "force-app/main/default/classes/GenerationProbe.cls")
	write := func(value string) {
		t.Helper()
		source := "public class GenerationProbe { public static String value() { return '" + value + "'; } }"
		if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("old")
	runner := NewRunner(ws, RunnerOptions{Version: "test"})
	loads := 0
	runner.loadWorkspaceIndex = func(root string) (typesys.Index, []diagnostic.Diagnostic, error) {
		index, diagnostics, err := loadWorkspaceIndex(root)
		loads++
		if loads == 1 {
			write("new")
		}
		return index, diagnostics, err
	}
	result, err := runner.Run(t.Context(), RunRequest{AnonymousBody: "System.debug(GenerationProbe.value());"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RunStatusPass || len(result.Logs) != 1 || result.Logs[0] != "new" || loads < 2 {
		t.Fatalf("result=%#v, loads=%d; want latest source and a retry", result, loads)
	}
}
