package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A plain Custom Error returns a row failure on a partial singleton insert.
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
			program, err := CompileAnonymousWithOptions(`Database.SaveResult r = Database.insert(new Account(Name='Example'), false); System.assertEquals(false, r.isSuccess()); System.assertEquals('plain uncaptured error', r.getErrors()[0].getMessage());`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			if err != nil {
				t.Fatalf("partial result: %v", err)
			}
		})
	}
}
