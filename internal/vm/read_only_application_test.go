package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
)

// Owned source-61/62/67 observations cover partial delete failure and recovery
// after disabling test-only application maintenance mode.
func TestReadOnlyApplicationPartialDeleteAndReset(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			executeReadOnlyApplicationProbe(t, api, `
List<Account> rows = new List<Account>{new Account(Name='First'),new Account(Name='Second')};
insert rows;
Test.setReadOnlyApplicationMode(true);
List<Database.DeleteResult> denied = Database.delete(rows,false);
System.assertEquals(2,denied.size());
for(Database.DeleteResult result:denied) {
    System.assertEquals(false,result.isSuccess());
    System.assertNotEquals(null,result.getId());
    System.assertEquals(1,result.getErrors().size());
    System.assertEquals('INSERT_UPDATE_DELETE_NOT_ALLOWED_DURING_MAINTENANCE',String.valueOf(result.getErrors()[0].getStatusCode()));
    System.assertEquals('Updates can’t be made during maintenance. Try again when maintenance is complete.',result.getErrors()[0].getMessage());
    System.assertEquals(0,result.getErrors()[0].getFields().size());
}
System.assertEquals(2,[SELECT COUNT() FROM Account]);
Test.setReadOnlyApplicationMode(false);
List<Database.DeleteResult> allowed = Database.delete(rows,false);
System.assertEquals(2,allowed.size());
System.assertEquals(true,allowed[0].isSuccess());
System.assertEquals(true,allowed[1].isSuccess());
System.assertEquals(0,[SELECT COUNT() FROM Account]);
`)
		})
	}
}

func TestReadOnlyApplicationModeControls(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			executeReadOnlyApplicationProbe(t, api, `
System.assertEquals('DEFAULT',String.valueOf(System.getApplicationReadWriteMode()));
Test.setReadOnlyApplicationMode(true);
System.assertEquals('READ_ONLY',String.valueOf(System.getApplicationReadWriteMode()));
Test.setReadOnlyApplicationMode(false);
System.assertEquals('DEFAULT',String.valueOf(System.getApplicationReadWriteMode()));
`)
		})
	}
}

func TestReadOnlyApplicationSetterRequiresTestContext(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions("Test.setReadOnlyApplicationMode(true);", CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			_, err = New(nil).Execute(program)
			want := "System.StringException: Test.setReadOnlyApplicationMode() can only be called from testMethods"
			if err == nil || err.Error() != want {
				t.Fatalf("anonymous boundary = %v, want %q", err, want)
			}
		})
	}
}

func executeReadOnlyApplicationProbe(t *testing.T, api, source string) {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestReadOnlyApplicationFailureResultID(t *testing.T) {
	for _, operation := range []string{"upsert", "undelete"} {
		t.Run(operation, func(t *testing.T) {
			const id = "001000000000001AAA"
			result := dml.Result{ID: id, StatusCode: "INSERT_UPDATE_DELETE_NOT_ALLOWED_DURING_MAINTENANCE"}
			got := databaseDMLResultIDValue(operation, result)
			if got.Kind == ValueNull || got.Text != id {
				t.Fatalf("maintenance result Id=%v, want %s", got, id)
			}
			result.StatusCode = "FIELD_CUSTOM_VALIDATION_EXCEPTION"
			if got := databaseDMLResultIDValue(operation, result); got.Kind != ValueNull {
				t.Fatalf("other failure result Id=%v, want null", got)
			}
		})
	}
}
