package gladecli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/dap"
	"github.com/glade-sh/glade/internal/vm"
)

func TestEditorLatencyDAPHarness(t *testing.T) {
	raw := os.Getenv("GLADE_EDITOR_LATENCY")
	if raw == "" {
		t.Skip("set GLADE_EDITOR_LATENCY to measure")
	}
	classes, err := strconv.Atoi(raw)
	if err != nil || classes < 1 {
		t.Fatal("invalid class count")
	}
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restore := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restore)
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "xdgcache"))
	project := filepath.Join(root, "project")
	writeGeneratedLatencyProject(t, project, classes)
	writeTestFile(t, filepath.Join(project, "force-app/main/classes/LatencyTest.cls"), `@isTest private class LatencyTest { @isTest static void runTest() { System.assert(true); } }`)
	launch := dap.LaunchRequest{ClassName: "LatencyTest", MethodName: "runTest", AnonymousBody: "System.debug(1);"}
	measure := func(name string, f func() error) {
		t.Helper()
		start := time.Now()
		if err := f(); err != nil {
			t.Fatal(err)
		}
		t.Logf("latency %s %s", name, time.Since(start))
	}
	measure("dap_test_launch_cold", func() error {
		return handleDAPLaunch(context.Background(), dap.NewHandler(dap.Snapshot{}), launch, project, "", true)
	})
	if _, err := dapTestIndexLoader(project); err != nil {
		t.Fatal(err)
	}
	measure("dap_test_launch_warm", func() error {
		return handleDAPLaunch(context.Background(), dap.NewHandler(dap.Snapshot{}), launch, project, "", true)
	})
	values := make([]vm.Value, 100000)
	for i := range values {
		values[i] = vm.Int(int64(i))
	}
	h := dap.NewHandler(dap.Snapshot{Vars: map[string]vm.Value{"items": vm.List(values...)}})
	args, _ := json.Marshal(map[string]any{"variablesReference": 1})
	measure("dap_variables_large_list", func() error {
		response := h.Handle(dap.Request{Seq: 1, Type: dap.MessageTypeRequest, Command: dap.CommandVariables, Arguments: args})[0].(dap.Response)
		if !response.Success {
			t.Fatal(response.Message)
		}
		return nil
	})
}
