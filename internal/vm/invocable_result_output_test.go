package vm

import "testing"

// RDTO01/02: hydrate the DTO first, then populate outputs with live Apex
// values. Populated Object-valued JSON maps remain rejected (RDTO03/the captured row).
func TestInvocableResultJSONOutputMapStaysOnResult(t *testing.T) {
	program, err := CompileAnonymous(`
Invocable.Action.Result dto=(Invocable.Action.Result)JSON.deserialize(
 '{"action":null,"errors":null,"success":true,"outputParameters":{}}',Invocable.Action.Result.class);
System.assertEquals(true,dto.isSuccess());
Map<String,Object> held=dto.getOutputParameters();
System.assertEquals(true,held.isEmpty());
Account output=new Account(Name='initial',Description='retained');
dto.getOutputParameters().putAll(new Map<String,Object>{'entry'=>output});
Invocable.Action.Result alias=dto;
List<Invocable.Action.Result> results=new List<Invocable.Action.Result>{alias};
System.assert(held===results[0].getOutputParameters());
System.assert(output===(Account)results[0].getOutputParameters().get('entry'));
output.Name='changed'; output.Description=null;
Account returned=(Account)alias.getOutputParameters().get('entry');
Account target=new Account(Name='original',Description='populated');
for(String fieldName:new List<String>{'Name','Description'})target.put(fieldName,returned.get(fieldName));
System.assertEquals('changed',target.Name);
System.assertEquals(null,target.Description);
held.put('nil',null);
System.assert(alias.getOutputParameters().containsKey('nil'));
System.assertEquals(null,alias.getOutputParameters().get('nil'));
alias.getOutputParameters().clear();
System.assertEquals(true,held.isEmpty());
System.assertEquals(null,dto.getErrors());
System.assertEquals(null,dto.getInvocationParameters());
System.assertEquals(null,dto.getAction());
Invocable.Action.Result failed=(Invocable.Action.Result)JSON.deserialize(
 '{"action":null,"errors":[{"code":"SYNTHETIC","message":"declined"}],"success":false,"outputParameters":{}}',Invocable.Action.Result.class);
System.assertEquals(false,failed.isSuccess());
System.assertEquals(1,failed.getErrors().size());
System.assertEquals('SYNTHETIC',failed.getErrors()[0].getCode());
System.assertEquals('declined',failed.getErrors()[0].getMessage());
Invocable.Action.Error detail=(Invocable.Action.Error)JSON.deserialize(
 '{"code":"SECOND","message":"second detail"}',Invocable.Action.Error.class);
System.assertEquals('SECOND',detail.getCode());
System.assertEquals('second detail',detail.getMessage());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
