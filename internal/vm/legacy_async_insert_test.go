package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Source API59 preserves the captured failed-result contract. API60 introduces
// the documented external-object validation, independently observed at60/64.
func TestVersionedAccountAsyncInsertNative(t *testing.T) {
	for _, tc := range []struct{ api, mode string }{{"59.0", "USER_MODE"}, {"59.0", "SYSTEM_MODE"}, {"60.0", "USER_MODE"}, {"60.0", "SYSTEM_MODE"}, {"64.0", "USER_MODE"}} {
		t.Run(tc.api+"/"+tc.mode, func(t *testing.T) {
			check := `System.assertEquals(1,results.size());System.assertEquals(false,results[0].isSuccess());System.assertEquals(null,results[0].getId());System.assertEquals(1,results[0].getErrors().size());Database.Error e=results[0].getErrors()[0];System.assertEquals('CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY',String.valueOf(e.getStatusCode()));System.assertEquals('Asynchronous DML not allowed on Account',e.getMessage());System.assertEquals(0,e.getFields().size());System.assertEquals(null,Database.getAsyncLocator(results[0]));`
			call := fmt.Sprintf(`results=Database.insertAsync(draft,new OwnedAsyncCallback(),AccessLevel.%s);`, tc.mode)
			want := 1
			if tc.api != "59.0" {
				want = 0
				call = `Boolean caught=false;try{` + call + ` }catch(TypeException e){caught=true;System.assertEquals('Invalid entity type in input for this operation',e.getMessage());}System.assert(caught);`
				check = `System.assertEquals(0,results.size());`
			}
			body := fmt.Sprintf(`List<SObject> draft=new List<SObject>{new Account(Name='OwnedAsyncBoundary')};List<Database.SaveResult> results=new List<Database.SaveResult>();Test.startTest();%s%sSystem.assertEquals(null,draft[0].Id);System.assertEquals(0,OwnedAsyncCallback.calls);System.assertEquals(0,[SELECT COUNT() FROM Account WHERE Name='OwnedAsyncBoundary']);System.assertEquals(%d,Limits.getDmlStatements());System.assertEquals(%d,Limits.getDmlRows());Test.stopTest();System.assertEquals(0,OwnedAsyncCallback.calls);System.assertEquals(0,Limits.getDmlStatements());System.assertEquals(0,Limits.getDmlRows());return true;`, call, check, want, want)
			helper, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: tc.api})
			if err != nil {
				t.Fatal(err)
			}
			callback, err := CompileAnonymous(`OwnedAsyncCallback.calls++;`)
			if err != nil {
				t.Fatal(err)
			}
			caller, err := CompileAnonymousWithOptions(`System.assertEquals(true,OwnedAsyncHelper.run());`, CompileOptions{APIVersion: "64.0"})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "Account")
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			for _, class := range []Class{
				{Name: "OwnedAsyncCallback", APIVersion: tc.api, SuperClass: "DataSource.AsyncSaveCallback", StaticFields: map[string]Field{"calls": {Name: "calls", Type: "Integer", Static: true, Value: Int(0), InitialValue: Int(0)}}, Methods: map[string]Method{"processSave": {Name: "OwnedAsyncCallback.processSave", ClassName: "OwnedAsyncCallback", ReturnType: "void", APIVersion: tc.api, Params: []Param{{Name: "saved", Type: "Database.SaveResult"}}, Program: callback}}},
				{Name: "OwnedAsyncHelper", APIVersion: tc.api, Methods: map[string]Method{"run": {Name: "OwnedAsyncHelper.run", ClassName: "OwnedAsyncHelper", ReturnType: "Boolean", IsStatic: true, APIVersion: tc.api, Program: helper}}},
			} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := machine.Execute(caller); err != nil {
				t.Fatal(err)
			}
		})
	}
}
