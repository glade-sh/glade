package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Owned API59/60 helpers called by API64 named tests enqueue a single empty
// standard error event in a genuine List<SObject>. P006 separately captures
// anonymous rejection at API62/67/68; modern named and API61 are not asserted.
func TestLegacyStandardEventPublishNative(t *testing.T) {
	for _, api := range []string{"59.0", "60.0"} {
		t.Run(api, func(t *testing.T) {
			runLegacyStandardPublishHelper(t, api, `
List<SObject> events=new List<SObject>{new BatchApexErrorEvent()};
System.assertEquals(0,Limits.getDmlStatements());
System.assertEquals(0,Limits.getDmlRows());
Test.startTest();
List<Database.SaveResult> results=EventBus.publish(events);
System.assertEquals(1,results.size());
Database.SaveResult saved=results[0];
System.assertEquals(true,saved.isSuccess());
System.assertNotEquals(null,saved.getId());
String acknowledgment=String.valueOf(saved.getId());
System.assertEquals(18,acknowledgment.length());
System.assert(acknowledgment.startsWith('1QQ'));
System.assertEquals(saved.getId(),Id.valueOf(acknowledgment));
System.assertNotEquals(null,saved.getErrors());
System.assertEquals(1,saved.getErrors().size());
Database.Error detail=saved.getErrors()[0];
System.assertEquals('OPERATION_ENQUEUED',String.valueOf(detail.getStatusCode()));
System.assert(Pattern.matches('[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}',detail.getMessage()));
System.assertEquals(0,detail.getFields().size());
Map<String,Object> wire=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(saved));
System.assertEquals(new Set<String>{'id','success','errors'},wire.keySet());
System.assertEquals(1,Limits.getDmlStatements());
System.assertEquals(1,Limits.getDmlRows());
Test.stopTest();
System.assertEquals(0,Limits.getDmlStatements());
System.assertEquals(0,Limits.getDmlRows());
System.assertEquals(true,saved.isSuccess());
System.assertNotEquals(null,saved.getId());
return true;
`)
		})
	}
}

func TestLegacyStandardEventPublishAnonymousNativeControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
String r;
List<SObject> events=new List<SObject>{new BatchApexErrorEvent()};
Database.SaveResult s=EventBus.publish(events)[0];
r=s.isSuccess()+'|'+(s.getId()==null)+'|'+s.getErrors().size();
if(!s.isSuccess()) {
 Database.Error e=s.getErrors()[0];
 r+='|'+e.getStatusCode()+'|'+e.getMessage()+'|'+String.join(e.getFields(),',');
}
System.assertEquals('false|true|1|CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY|DML operation INSERT not allowed on BatchApexErrorEvent|',r);
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "BatchApexErrorEvent")
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func runLegacyStandardPublishHelper(t *testing.T, api, body string) {
	t.Helper()
	helper, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	caller, err := CompileAnonymousWithOptions(`System.assertEquals(true,LegacyEventHelper.run());`, CompileOptions{APIVersion: "64.0"})
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "BatchApexErrorEvent")
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "LegacyEventHelper", APIVersion: api, Methods: map[string]Method{
		"run": {Name: "LegacyEventHelper.run", ClassName: "LegacyEventHelper", ReturnType: "Boolean", IsStatic: true, APIVersion: api, Program: helper},
	}}); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	if _, err := machine.Execute(caller); err != nil {
		t.Fatal(err)
	}
}

// Native standard_event_route_twins rows capture concrete and multi-event
// named lists at both source APIs; these follow the same gate as a scalar.
func TestNamedStandardEventPublishNativeListShapes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		success := "false"
		if api == "62.0" {
			success = "true"
		}
		for _, shape := range []struct{ name, source string }{
			{"concrete single", `new List<BatchApexErrorEvent>{new BatchApexErrorEvent()}`},
			{"erased multi", `new List<SObject>{new BatchApexErrorEvent(),new BatchApexErrorEvent()}`},
		} {
			t.Run(api+"/"+shape.name, func(t *testing.T) {
				runLegacyStandardPublishHelper(t, api, `List<SObject> events=`+shape.source+`;
List<Database.SaveResult> results=EventBus.publish(events);
System.assertEquals(events.size(),results.size());
for(Database.SaveResult saved:results) System.assertEquals(`+success+`,saved.isSuccess());
return true;
`)
			})
		}
	}
}
