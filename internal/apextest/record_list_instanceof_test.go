package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// A matcher written as `arg != null && arg instanceof SObject` receives a
// record list as an Object argument. instanceof SObject answers true for a
// List<SObject> or List<Account>, empty or not, while the explicit cast to
// SObject still throws.
func TestRunRecordListHeldAsObjectIsInstanceOfSObject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion": "62.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/RowMatchProof.cls"), `@IsTest private class RowMatchProof {
    private interface ArgCheck { Boolean accepts(Object arg); }
    private class AnyRecord implements ArgCheck {
        public Boolean accepts(Object arg) { return arg != null && arg instanceof SObject; }
    }
    private static String castText(Object held) {
        try { SObject row = (SObject)held; return 'cast'; }
        catch (TypeException e) { return e.getMessage(); }
    }
    @IsTest static void recordListsMatchAndCastStillThrows() {
        ArgCheck check = new AnyRecord();
        Map<Id, Account> byId = new Map<Id, Account>{ '001000000000001AAA' => new Account(Id = '001000000000001AAA') };
        Object fromValues = byId.values();
        Object typed = new List<Account>{ new Account(Name = 'a') };
        Object generic = new List<SObject>{ new Account(Name = 'b') };
        Object emptyTyped = new List<Account>();
        Object emptyGeneric = new List<SObject>();
        Object plain = new List<Object>{ new Account(Name = 'c') };
        System.assertEquals(true, check.accepts(fromValues));
        System.assertEquals(true, check.accepts(typed));
        System.assertEquals(true, check.accepts(generic));
        System.assertEquals(true, emptyTyped instanceof SObject);
        System.assertEquals(true, emptyGeneric instanceof SObject);
        System.assertEquals(false, plain instanceof SObject);
        System.assertEquals(false, check.accepts(null));
        System.assertEquals('Invalid conversion from runtime type List<Account> to SObject', castText(typed));
        System.assertEquals('Invalid conversion from runtime type List<SObject> to SObject', castText(generic));
    }
}`)
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("record list instanceof contract failed: %s", data)
	}
}
