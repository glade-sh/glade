package vm

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestApprovalProcessMixedListSubmissionAndWorkitemRoutes(t *testing.T) {
	for _, route := range []string{"submission", "workitem"} {
		for _, firstFailure := range []int{0, 1} {
			for _, variant := range []string{"default", "true", "false", "success"} {
				t.Run(fmt.Sprintf("%s/first_failure_%d/%s", route, firstFailure, variant), func(t *testing.T) {
					machine := New(nil)
					org := testDataOrg()
					testSeedApprovalMetadata(t, &org)
					machine.SetOrg(&org)
					requests := typedList("List<Approval.ProcessRequest>")
					for i := 0; i < 4; i++ {
						id := storage.ID(fmt.Sprintf("00100000000010%dAAA", i))
						testSeedApprovalAccount(t, &org, id, "Owned mixed approval")
						request := Object("Approval.ProcessSubmitRequest")
						request.Fields["ObjectId"] = platformScalar("Id", string(id))
						request.Fields["SkipEntryCriteria"] = Bool(true)
						bad := variant != "success" && i%2 == firstFailure
						if route == "workitem" {
							pending, err := machine.executeApprovalProcess([]Value{request})
							if err != nil {
								t.Fatal(err)
							}
							request = Object("Approval.ProcessWorkitemRequest")
							request.Fields["WorkitemId"] = pending.Fields["newWorkitemIds"].List[0]
							request.Fields["Action"] = String("Approve")
							if bad {
								request.Fields["Action"] = String("Invalid")
							}
						} else if bad {
							request.Fields["ProcessDefinitionNameOrId"] = String("No_Such_Process")
						}
						requests.List = append(requests.List, request)
					}
					journal := machine.isolationJournal
					args := []Value{requests}
					if variant == "true" || variant == "false" {
						args = append(args, Bool(variant == "true"))
					}
					result, err := machine.executeApprovalProcess(args)
					if variant == "default" || variant == "true" {
						var thrown *apexThrowError
						if !errors.As(err, &thrown) {
							t.Fatalf("error = %v, want DmlException", err)
						}
						details := thrown.value.Fields["__dmlErrors"].List
						if len(details) != 2 {
							t.Fatalf("DML failure count = %d, want 2 (request indices %d and %d)", len(details), firstFailure, firstFailure+2)
						}
						for i, detail := range details {
							if got, want := detail.Fields["index"].Int, int64(firstFailure+2*i); got != want {
								t.Errorf("failure %d index = %d, want %d", i, got, want)
							}
						}
					} else {
						if err != nil {
							t.Fatal(err)
						}
						if len(result.List) != 4 {
							t.Fatalf("result count = %d, want 4", len(result.List))
						}
						for i, item := range result.List {
							want := variant == "success" || i%2 != firstFailure
							if got := item.Fields["success"].Bool; got != want {
								t.Errorf("request %d success = %v, want %v", i, got, want)
							}
						}
					}
					if machine.isolationJournal != journal {
						t.Error("list call retained its temporary rollback journal")
					}
					instances, approved, workitems, steps := 0, 0, 0, 0
					for _, instance := range org.Objects["ProcessInstance"].Records {
						instances++
						if storageStringField(instance, "Status") == "Approved" {
							approved++
						}
					}
					for _, work := range org.Objects["ProcessInstanceWorkitem"].Records {
						if storageStringField(work, "IsDeleted") != "true" {
							workitems++
						}
					}
					steps = len(org.Objects["ProcessInstanceStep"].Records)
					wantSuccesses := 0
					if variant == "false" {
						wantSuccesses = 2
					} else if variant == "success" {
						wantSuccesses = 4
					}
					wantInstances, wantApproved, wantWorkitems, wantSteps := wantSuccesses, 0, wantSuccesses, wantSuccesses
					if route == "workitem" {
						wantInstances, wantApproved, wantWorkitems, wantSteps = 4, wantSuccesses, 4-wantSuccesses, 4+wantSuccesses
					}
					if instances != wantInstances || approved != wantApproved || workitems != wantWorkitems || steps != wantSteps {
						t.Fatalf("state = %d instances, %d approved, %d workitems, %d steps; want %d, %d, %d, %d", instances, approved, workitems, steps, wantInstances, wantApproved, wantWorkitems, wantSteps)
					}
				})
			}
		}
	}
}

func TestApprovalProcessListPreservesCallerRollbackJournal(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	testSeedApprovalMetadata(t, &org)
	testSeedApprovalAccount(t, &org, "001000000000101AAA", "Owned outer transaction")
	machine.SetOrg(&org)
	outer := machine.beginDMLRollbackPoint(true, false)
	defer machine.finishDMLRollbackPoint(outer)
	journal := machine.isolationJournal
	request := Object("Approval.ProcessSubmitRequest")
	request.Fields["ObjectId"] = platformScalar("Id", "001000000000101AAA")
	request.Fields["SkipEntryCriteria"] = Bool(true)
	requests := typedList("List<Approval.ProcessRequest>")
	requests.List = append(requests.List, request)
	if _, err := machine.executeApprovalProcess([]Value{requests, Bool(true)}); err != nil {
		t.Fatal(err)
	}
	if machine.isolationJournal != journal {
		t.Fatal("successful list detached the caller's rollback journal")
	}
	if err := machine.restoreDMLRollbackPoint(outer); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ProcessInstance", "ProcessInstanceWorkitem", "ProcessInstanceStep"} {
		if got := len(org.Objects[name].Records); got != 0 {
			t.Errorf("outer rollback left %d %s records", got, name)
		}
	}
}
