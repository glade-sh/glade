package vm

import (
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The same anonymous source passed on Salesforce at both API endpoints,
// internal/sema/testdata/cart_orchestrator_visibility/org.tsv:12,14.
func TestCartOrchestratorDirectAnonymousNativeRoutes(t *testing.T) {
	source, err := os.ReadFile("../sema/testdata/cart_orchestrator_visibility/direct.apex")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(string(source), CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetToolingExecuteAnonymous(true)
			for i := 0; i < 2; i++ {
				if _, err := machine.Execute(program); err != nil {
					t.Fatalf("pass %d: %v", i, err)
				}
			}
		})
	}
}
