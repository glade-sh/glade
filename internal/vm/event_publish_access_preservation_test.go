package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Local preservation controls: these routes have no corresponding native rows
// in event_publish_entity_access and retain their pre-change behavior.
func TestEventPublishUncapturedRoutesPreserved(t *testing.T) {
	for _, route := range []string{"access_level", "replay", "outside_runas"} {
		t.Run(route, func(t *testing.T) {
			machine := fieldPermissionFallbackMachine(false)
			machine.Org.Objects["ProbeSignal__e"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "ProbeSignal__e", KeyPrefix: "e01", Metadata: map[string]string{"publishBehavior": "PublishImmediately"}, Fields: map[string]storage.Field{}}, Records: map[storage.ID]storage.Record{}}
			actor := machine.executionUser
			machine.EnableTestContext()
			machine.testContext.CurrentUser = actor
			machine.testContext.RunAsDepth = 1
			machine.currentMethod = Method{APIVersion: "67.0"}
			var result Value
			var err error
			switch route {
			case "access_level":
				result, err = machine.eventBusPublishWithAccessLevel([]Value{Object("ProbeSignal__e"), Object("AccessLevel")}, &Result{})
			case "replay":
				result, err = machine.eventBusPublishWithNamedSource([]Value{Object("ProbeSignal__e")}, &Result{}, true, false)
			case "outside_runas":
				machine.testContext.RunAsDepth = 0
				result, err = machine.eventBusPublish([]Value{Object("ProbeSignal__e")}, &Result{})
			}
			if err != nil || !result.Fields["success"].Bool {
				t.Fatalf("preserved route %s result=%+v err=%v", route, result, err)
			}
		})
	}
}

// capture_09-10 establish that a permission-set create grant permits publish.
func TestEventPublishPermissionSetGrant(t *testing.T) {
	machine := fieldPermissionFallbackMachine(false)
	name := "ProbeSignal__e"
	machine.Org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, KeyPrefix: "e01", Metadata: map[string]string{"publishBehavior": "PublishImmediately"}, Fields: map[string]storage.Field{}}, Records: map[storage.ID]storage.Record{}}
	perms := machine.Org.Objects["ObjectPermissions"]
	perms.Records["110000000000098"] = storage.Record{ID: "110000000000098", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000099"), "SObjectType": storage.StringValue(name), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true)}}
	machine.Org.Objects["ObjectPermissions"] = perms
	assignments := machine.Org.Objects["PermissionSetAssignment"]
	assignments.Records["0Pa000000000099"] = storage.Record{ID: "0Pa000000000099", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue("005000000000099"), "PermissionSetId": storage.IDValue("0PS000000000099")}}
	machine.Org.Objects["PermissionSetAssignment"] = assignments
	actor := machine.executionUser
	machine.EnableTestContext()
	machine.testContext.CurrentUser = actor
	machine.testContext.RunAsDepth = 1
	machine.currentMethod = Method{APIVersion: "67.0"}
	result, err := machine.eventBusPublish([]Value{Object(name)}, &Result{})
	if err != nil || !result.Fields["success"].Bool {
		t.Fatalf("assigned grant result=%+v err=%v", result, err)
	}
}
