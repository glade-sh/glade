package vm

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// Local state-preservation checks complement the captured source rows.
func TestB44XEventEntryRestoresCaller(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, bound := range []bool{false, true} {
			for _, declaration := range []string{"", "with sharing", "without sharing", "inherited sharing"} {
				for _, failureCallback := range []bool{false, true} {
					for _, throws := range []bool{false, true} {
						t.Run(fmt.Sprintf("%s/bound=%t/%s/failure=%t/throws=%t", api, bound, declaration, failureCallback, throws), func(t *testing.T) {
							machine := New(io.Discard)
							org := privateExecutionPolicyOrg()
							org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
								"005000000000002": {ID: "005000000000002", Object: "User", Fields: map[string]storage.Value{"UserType": storage.StringValue("AutomatedProcess")}},
							}}
							machine.SetOrg(&org)
							machine.EnableTestContext()
							caller := Object("User")
							caller.Fields["Id"] = String("005000000000001")
							caller.Fields["UserType"] = String("Standard")
							machine.testContext.CurrentUser = caller
							machine.testContext.RunAsDepth = 1
							machine.eventPublishReadUserID = "outer"
							wantUser := "Standard"
							if declaration == "" && !failureCallback {
								wantUser = "AutomatedProcess"
							}
							source := "System.assertEquals('" + wantUser + "', UserInfo.getUserType());"
							if throws {
								source += "throw new IllegalArgumentException('callback failure');"
							}
							program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							if err := machine.RegisterClass(Class{Name: "Callback", APIVersion: api, Modifiers: []string{declaration}}); err != nil {
								t.Fatal(err)
							}
							method := Method{Name: "Callback.onSuccess", ClassName: "Callback", APIVersion: api, SourceContextBound: bound, Program: program}
							if bound {
								method.SharingMode = declaration
							}
							_, err = machine.callEventPublishEntry(method, Object("Callback"), nil, &Result{}, failureCallback)
							if throws {
								if err == nil || !strings.Contains(err.Error(), "callback failure") {
									t.Fatalf("error = %v", err)
								}
							} else if err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(machine.testContext.CurrentUser, caller) || machine.eventPublishReadUserID != "outer" || machine.testContext.RunAsDepth != 1 {
								t.Fatal("callback did not restore caller context")
							}
						})
					}
				}
			}
		}
	}
}

func TestB44XEventReadScopePreservesOtherSecurityModesAndUsers(t *testing.T) {
	for _, test := range []struct {
		name, mode, user, scope string
		want                    int
	}{
		{"captured system read", "SYSTEM_MODE", "005000000000002", "005000000000002", 2},
		{"user mode unchanged", "USER_MODE", "005000000000002", "005000000000002", 1},
		{"nested runAs unchanged", "SYSTEM_MODE", "005000000000001", "005000000000002", 1},
		{"outside callback unchanged", "SYSTEM_MODE", "005000000000002", "", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			machine := New(io.Discard)
			org := privateExecutionPolicyOrg()
			machine.SetOrg(&org)
			machine.EnableTestContext()
			machine.testContext.CurrentUser = Object("User")
			machine.testContext.CurrentUser.Fields["Id"] = String(test.user)
			machine.testContext.RunAsDepth = 1
			machine.currentClass = "WithHelper"
			machine.currentMethod = Method{SourceContextBound: true, SharingMode: "with sharing", APIVersion: "67.0"}
			machine.eventPublishReadUserID = test.scope
			var records []storage.Record
			for _, record := range org.Objects["Widget__c"].Records {
				records = append(records, record)
			}
			got := machine.applySOQLSharing(soql.Query{Object: "Widget__c", SecurityMode: test.mode}, soql.Result{Records: records, Rows: len(records)})
			if len(got.Records) != test.want {
				t.Fatalf("visible rows = %d, want %d", len(got.Records), test.want)
			}
		})
	}
}
