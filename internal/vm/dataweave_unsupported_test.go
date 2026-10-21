package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDataWeaveUnavailableReportsUnsupported(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	program, err := CompileAnonymous(`DataWeave.Script.createScript('greeting').execute();`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Metadata.DataWeaveResources = []storage.DataWeaveResourceMetadata{{
		Name: "greeting", Content: "%dw 2.0\noutput application/json\n---\n'hello'",
		APIVersion: "62.0", ContentPath: "force-app/main/default/dw/greeting.dwl",
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || !strings.Contains(err.Error(), "DataWeave is unsupported") {
		t.Fatalf("error = %v, want unsupported DataWeave diagnostic", err)
	}
	if strings.Contains(err.Error(), "glade toolchain install") {
		t.Fatalf("excluded component suggests installation: %v", err)
	}
}
