package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Plain Custom Error messages use the captured singleton SaveResult route.
func TestFlowPartialPlainSingletonReturnsResult(t *testing.T) {
	for _, message := range []string{"unmatched plain error", "another unrelated error"} {
		t.Run(message, func(t *testing.T) {
			org := testDataOrg()
			account := org.Objects["Account"]
			account.Definition.FlowRules = []storage.FlowRule{{Name: "Reject", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Steps: []storage.FlowStep{{Kind: "customError", CustomError: storage.FlowCustomError{Messages: []storage.FlowCustomErrorMessage{{Message: message}}}}}}}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)
			program, err := CompileAnonymousWithOptions(`Database.SaveResult r = Database.insert(new Account(Name='Example'), false); System.assertEquals(false, r.isSuccess());`, CompileOptions{APIVersion: "67.0"})
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
