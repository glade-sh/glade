package vm

import (
	"errors"
	"fmt"
	"testing"
)

// The single_* and mixed_workitems/mixed_reject native rows cover these routes.
func TestApprovalWorkitemSingleAndListNativeFields(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"single_request", "base_request_list", "typed_workitem_list"} {
			for _, variant := range []string{"default", "true", "false"} {
				for _, action := range []string{"Approve", "Reject", "Invalid"} {
					t.Run(fmt.Sprintf("api%s/%s/%s/%s", api, route, variant, action), func(t *testing.T) {
						machine := New(nil)
						org := testDataOrg()
						org.APIVersion = api
						testSeedApprovalMetadata(t, &org)
						testSeedApprovalAccount(t, &org, "001000000000101AAA", "Owned workitem fields")
						machine.SetOrg(&org)
						submit := Object("Approval.ProcessSubmitRequest")
						submit.Fields["ObjectId"] = platformScalar("Id", "001000000000101AAA")
						submit.Fields["SkipEntryCriteria"] = Bool(true)
						pending, err := machine.executeApprovalProcess([]Value{submit})
						if err != nil {
							t.Fatal(err)
						}
						request := Object("Approval.ProcessWorkitemRequest")
						request.Fields["WorkitemId"] = pending.Fields["newWorkitemIds"].List[0]
						request.Fields["Action"] = String(action)
						input := request
						if route != "single_request" {
							input = typedList("List<Approval.ProcessRequest>")
							if route == "typed_workitem_list" {
								input = typedList("List<Approval.ProcessWorkitemRequest>")
							}
							input.List = append(input.List, request)
						}
						args := []Value{input}
						if variant != "default" {
							args = append(args, Bool(variant == "true"))
						}
						result, err := machine.executeApprovalProcess(args)
						if action == "Invalid" && variant != "false" {
							var thrown *apexThrowError
							if !errors.As(err, &thrown) {
								t.Fatalf("error = %v, want native DmlException", err)
							}
							const message = "Process failed. First exception on row 0; first error: INVALID_OPERATION, Illegal transition type: []"
							if got := exceptionMessage(thrown.value); got != message {
								t.Errorf("exception message = %q, want %q", got, message)
							}
							details := thrown.value.Fields["__dmlErrors"].List
							if len(details) != 1 {
								t.Errorf("DML details = %d, want 1", len(details))
							} else {
								assertWorkitemNativeError(t, details[0])
								if got := details[0].Fields["index"].Int; got != 0 {
									t.Errorf("DML index = %d, want 0", got)
								}
							}
						} else {
							if err != nil {
								t.Fatal(err)
							}
							if route != "single_request" {
								if len(result.List) != 1 {
									t.Fatalf("result count = %d, want 1", len(result.List))
								}
								result = result.List[0]
							}
							if got, want := result.Fields["success"].Bool, action != "Invalid"; got != want {
								t.Errorf("success = %v, want %v", got, want)
							}
							problems := result.Fields["errors"].List
							if action == "Invalid" {
								if len(problems) != 1 {
									t.Fatalf("error count = %d, want 1", len(problems))
								}
								assertWorkitemNativeError(t, problems[0])
							} else if len(problems) != 0 {
								t.Errorf("successful action errors = %v", problems)
							}
						}
						work := testOnlyRecord(t, &org, "ProcessInstanceWorkitem")
						wantDeleted := action != "Invalid"
						if got := work.System.IsDeleted; got != wantDeleted {
							t.Errorf("storage deletion flag = %v, want %v (SOQL live workitem visibility)", got, wantDeleted)
						}
						if got := storageStringField(work, "IsDeleted"); got != fmt.Sprint(wantDeleted) {
							t.Errorf("IsDeleted field = %q, want %v", got, wantDeleted)
						}
					})
				}
			}
		}
	}
}

func assertWorkitemNativeError(t *testing.T, problem Value) {
	t.Helper()
	if got := problem.Fields["statusCode"].Text; got != "INVALID_OPERATION" {
		t.Errorf("status = %q, want INVALID_OPERATION", got)
	}
	if got := problem.Fields["message"].Text; got != "Illegal transition type" {
		t.Errorf("message = %q, want Illegal transition type", got)
	}
	if got := problem.Fields["fields"]; got.Kind != ValueList || len(got.List) != 0 {
		t.Errorf("fields = %v, want empty list", got)
	}
}
