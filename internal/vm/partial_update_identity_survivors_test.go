package vm

import (
	"fmt"
	"testing"
)

// allOrNone=false commits the successful input records and returns a result
// per record. Owned A36 R130 admits mixed update results; deleted identity
// classification itself is unchanged by this survivor-index regression.
// https://trailhead.salesforce.com/content/learn/modules/apex_database/apex_database_dml
func TestExecPartialUpdateIdentitySurvivors(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, position := range []int{0, 1, 2} {
			for _, overload := range []string{"false", "options"} {
				t.Run(fmt.Sprintf("%s/%d/%s", api, position, overload), func(t *testing.T) {
					program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Account gone=new Account(Name='gone');Account first=new Account(Name='first');Account second=new Account(Name='second');
insert new List<Account>{gone,first,second};delete gone;
List<Account> inputs=new List<Account>{first,second};inputs.add(%d,gone);
for(Account value:inputs)value.Name='changed';
Database.DMLOptions options=new Database.DMLOptions();options.OptAllOrNone=false;
List<Database.SaveResult> results=Database.update(inputs,%s);
System.assertEquals(3,results.size());
for(Integer i=0;i<inputs.size();i++){
 if(i==%d){
  System.assertEquals(false,results[i].isSuccess());
  System.assertEquals(false,results[i].getErrors().isEmpty());
  System.assertEquals(StatusCode.ENTITY_IS_DELETED,results[i].getErrors()[0].getStatusCode());
 }else{
  System.assertEquals(true,results[i].isSuccess());
  System.assertEquals(inputs[i].Id,results[i].getId());
  System.assertEquals(0,results[i].getErrors().size());
 }
}
System.assertEquals(2,[SELECT COUNT() FROM Account WHERE Name='changed']);
System.assertEquals(0,[SELECT COUNT() FROM Account WHERE Id=:gone.Id]);
`, position, overload, position), CompileOptions{APIVersion: api})
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
	}
}

func TestExecPartialIdentityAndTriggerFailuresRetainPositions(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	trigger, err := CompileAnonymous(`for(Account value:Trigger.new){if(value.Name=='blocked')value.addError('blocked input');}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{Name: "SurvivorGuard", Object: "Account", Operation: "update", Timing: triggerTimingBefore, Program: trigger}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account gone=new Account(Name='gone');Account blocked=new Account(Name='blocked before');Account live=new Account(Name='live before');
insert new List<Account>{gone,blocked,live};delete gone;
gone.Name='changed';blocked.Name='blocked';live.Name='changed';
List<Database.SaveResult> results=Database.update(new List<Account>{gone,blocked,live},false);
System.assertEquals(3,results.size());
System.assertEquals(false,results[0].isSuccess());
System.assertEquals(StatusCode.ENTITY_IS_DELETED,results[0].getErrors()[0].getStatusCode());
System.assertEquals(false,results[1].isSuccess());
System.assertEquals('blocked input',results[1].getErrors()[0].getMessage());
System.assertEquals(true,results[2].isSuccess());
System.assertEquals('blocked before',[SELECT Name FROM Account WHERE Id=:blocked.Id].Name);
System.assertEquals('changed',[SELECT Name FROM Account WHERE Id=:live.Id].Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAllOrNoneIdentityFailurePreservesLiveRecord(t *testing.T) {
	program, err := CompileAnonymous(`
Account gone=new Account(Name='gone');Account live=new Account(Name='live');
insert new List<Account>{gone,live};delete gone;
gone.Name='changed';live.Name='changed';
Boolean caught=false;
try{Database.update(new List<Account>{gone,live},true);}catch(DmlException expected){caught=true;}
System.assertEquals(true,caught);
System.assertEquals('live',[SELECT Name FROM Account WHERE Id=:live.Id].Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
