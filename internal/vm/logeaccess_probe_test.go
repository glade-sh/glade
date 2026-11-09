package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// event_publish_entity_access/org.tsv capture_01-10: named publishers inside
// System.runAs at API 62/66/67/68, including permission-set controls.
func TestEventPublishEntityAccessProbe(t *testing.T) {
	for _, api := range []string{"62.0", "66.0", "67.0", "68.0"} {
		for _, access := range []string{"implicit", "none", "read", "create"} {
			for _, name := range []string{"ProbeSignal__e", "AlertSignal__e"} {
				for _, shape := range []string{"scalar", "typed", "generic"} {
					t.Run(api+"/"+access+"/"+name+"/"+shape, func(t *testing.T) {
						machine := fieldPermissionFallbackMachine(false)
						machine.Org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, KeyPrefix: "e01", Metadata: map[string]string{"publishBehavior": "PublishImmediately"}, Fields: map[string]storage.Field{}}, Records: map[storage.ID]storage.Record{}}
						perms := machine.Org.Objects["ObjectPermissions"]
						if access != "implicit" {
							perms.Records["110000000000098"] = storage.Record{ID: "110000000000098", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("00e000000000005"), "SObjectType": storage.StringValue(name), "PermissionsRead": storage.BooleanValue(access != "none"), "PermissionsCreate": storage.BooleanValue(access == "create")}}
						}
						machine.Org.Objects["ObjectPermissions"] = perms
						actor := machine.executionUser
						machine.EnableTestContext()
						machine.testContext.CurrentUser = actor
						machine.testContext.RunAsDepth = 1
						machine.currentMethod = Method{APIVersion: api}
						input := Object(name)
						if shape != "scalar" {
							input = List(input)
							if shape == "typed" {
								input.Type = "List<" + name + ">"
							} else {
								input.Type = "List<SObject>"
							}
						}
						got, err := machine.eventBusPublish([]Value{input}, &Result{})
						if err != nil {
							t.Fatalf("publish threw: %v", err)
						}
						if shape != "scalar" {
							if len(got.List) != 1 {
								t.Fatalf("unexpected list result: %+v", got)
							}
							got = got.List[0]
						}
						wantSuccess := api == "62.0" || api == "66.0" || access == "create"
						if got.Fields["success"].Bool != wantSuccess {
							t.Fatalf("success=%v, want %v", got.Fields["success"].Bool, wantSuccess)
						}
						if wantSuccess {
							return
						}
						message := fmt.Sprintf("Access to entity '%s' denied", name)
						if access == "read" {
							message = fmt.Sprintf("DML operation INSERT not allowed on %s", name)
						}
						errors := got.Fields["errors"].List
						if len(errors) != 1 || errors[0].Fields["statusCode"].String() != "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY" || errors[0].Fields["message"].String() != message {
							t.Fatalf("errors=%+v, want %q", errors, message)
						}
					})
				}
			}
		}
	}
}
