package vm

import (
	"testing"
)

// Reduction checks only whether a partial batch update classifies a deleted
// identity. Retry-queue/log lifecycle is separately unprobed.
func TestExecBatchDeletedParentPartialUpdate(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			machine.EnableTestContext()
			start, err := CompileAnonymous(`return new List<SObject>{new Account(Id=CleanupBatch.goneId,Name='changed'),new Account(Id=CleanupBatch.liveId,Name='changed')};`)
			if err != nil {
				t.Fatal(err)
			}
			execute, err := CompileAnonymous(`
Database.DMLOptions options=new Database.DMLOptions();
options.DuplicateRuleHeader.allowSave=true;
options.DuplicateRuleHeader.runAsCurrentUser=true;
options.OptAllOrNone=false;
List<Database.SaveResult> results=Database.update(scope,options);
System.debug('SCOPE|'+JSON.serialize(scope));
System.debug('RESULTS|'+JSON.serialize(results));
System.assertEquals(2,results.size());
System.assertEquals(false,results[0].isSuccess());
System.assertEquals(true,results[1].isSuccess());
Boolean deleted=false;
for(Database.Error problem:results[0].getErrors()) {
 System.debug('DELETED_ERROR|'+problem.getStatusCode()+'|'+problem.getMessage());
 System.assertEquals('ENTITY_IS_DELETED',String.valueOf(problem.getStatusCode()));
 if(problem.getStatusCode()==StatusCode.ENTITY_IS_DELETED) deleted=true;
}
System.assertEquals(true,deleted);
`)
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterClass(Class{Name: "CleanupBatch", Modifiers: []string{"without sharing"}, Interfaces: []string{"Database.Batchable<SObject>"}, StaticFields: map[string]Field{"goneId": {Name: "goneId", Type: "Id", Static: true}, "liveId": {Name: "liveId", Type: "Id", Static: true}}, Methods: map[string]Method{
				"start":   {Name: "CleanupBatch.start", ClassName: "CleanupBatch", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: start},
				"execute": {Name: "CleanupBatch.execute", ClassName: "CleanupBatch", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: execute},
				"finish":  {Name: "CleanupBatch.finish", ClassName: "CleanupBatch", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
			}}); err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymousWithOptions(`
Account gone=new Account(Name='gone');Account live=new Account(Name='live');
insert new List<Account>{gone,live};
CleanupBatch.goneId=gone.Id;CleanupBatch.liveId=live.Id;
delete gone;
Test.startTest();Database.executeBatch(new CleanupBatch(),100);Test.stopTest();
System.assertEquals('changed',[SELECT Name FROM Account WHERE Id=:live.Id].Name);
System.assertEquals(0,[SELECT COUNT() FROM Account WHERE Id=:gone.Id]);
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			for _, line := range result.Debug {
				t.Log(line)
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
