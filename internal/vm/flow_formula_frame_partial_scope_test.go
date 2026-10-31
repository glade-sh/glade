package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The capture only covers formula-bearing before-save Create errors. A plain
// unrelated Custom Error keeps the partial-insert behavior of the base tree.
func TestFlowFormulaFramePartialInsertUncapturedError(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := testDataOrg()
			org.APIVersion = api
			account := org.Objects["Account"]
			account.Definition.FlowRules = []storage.FlowRule{{Name: "RejectPlain", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Steps: []storage.FlowStep{{Kind: "customError", CustomError: storage.FlowCustomError{Name: "Reject", Messages: []storage.FlowCustomErrorMessage{{Message: "plain uncaptured error"}}}}}}}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)
			program, err := CompileAnonymousWithOptions(`Database.insert(new Account(Name='Example'), false);`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			if err == nil || !strings.Contains(err.Error(), "plain uncaptured error") {
				t.Fatalf("uncaptured error changed: %v", err)
			}
		})
	}
}
