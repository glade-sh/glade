package vm

import (
	"strings"
	"testing"
)

// Owned API64 anonymous and named rows C1/C2 admit and serialize this opaque
// handle without resolving it to rows. C3 covers its list-backed wrapper use.
func TestCursorJSONShellAdmissionNativeAPI64(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Integer beforeQueries=Limits.getQueries();
Database.Cursor shell=(Database.Cursor)JSON.deserialize('{"queryId":"0r8Ab12Cd34Ef56"}',Database.Cursor.class);
System.assert(shell!=null);
System.assert(((Object)shell) instanceof Database.Cursor);
System.assertEquals('{"queryId":"0r8Ab12Cd34Ef56"}',JSON.serialize(shell));
System.assertEquals(beforeQueries,Limits.getQueries());
`, CompileOptions{APIVersion: "64.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if len(machine.queryCursors) != 0 {
		t.Fatal("opaque handle registered a local query snapshot")
	}
	for _, field := range []string{"Records", "Query"} {
		if _, found := machine.Globals["shell"].Fields[field]; found {
			t.Fatalf("opaque handle acquired invented %s", field)
		}
	}
}

func TestCursorJSONShellNativeControls(t *testing.T) {
	for name, source := range map[string]string{
		"empty": `Database.Cursor c=(Database.Cursor)JSON.deserialize('{}',Database.Cursor.class); System.assert(c!=null); System.assertEquals('{"queryId":null}',JSON.serialize(c));`,
		"null":  `System.assertEquals(null,JSON.deserialize('null',Database.Cursor.class));`,
		"registered roundtrip": `Account seed=new Account(Name='owned cursor json roundtrip'); insert seed; Id seedId=seed.Id;
Database.Cursor original=Database.getCursor('SELECT Id,Name FROM Account WHERE Id = :seedId');
Database.Cursor restored=(Database.Cursor)JSON.deserialize(JSON.serialize(original),Database.Cursor.class);
System.assertEquals(1,restored.getNumRecords()); List<SObject> rows=restored.fetch(0,1);
System.assertEquals(1,rows.size()); System.assertEquals(seed.Id,rows[0].Id); System.assertEquals(seed.Name,rows[0].get('Name'));`,
	} {
		t.Run(name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "64.0"})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Native fake-handle count/fetch remain unanswered. Retain the local explicit
// unsupported boundary rather than manufacture an empty result from the shell.
func TestCursorJSONShellRetainsUnresolvedMethodBoundary(t *testing.T) {
	for _, operation := range []string{"shell.getNumRecords()", "shell.fetch(0,1)"} {
		t.Run(operation, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Database.Cursor shell=(Database.Cursor)JSON.deserialize('{"queryId":"0r8Ab12Cd34Ef56"}',Database.Cursor.class);
`+operation+`;`, CompileOptions{APIVersion: "64.0"})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			_, err = machine.Execute(program)
			if err == nil || !strings.Contains(err.Error(), "queryId resolution outside the local execution") {
				t.Fatalf("got %v; want unresolved hosted-cursor boundary", err)
			}
			if machine.Globals["shell"].Kind != ValueObject {
				t.Fatal("admission failed before the unresolved method boundary")
			}
		})
	}
}
