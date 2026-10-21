@IsTest private class AsyncValidationTest {
 private static Map<String,Object> observe(String shape,Boolean userMode) {
  Account draft=new Account(Name='Async validation');
  Boolean bounded=shape!='outside';
  if(bounded) Test.startTest();
  Integer statements=Limits.getDmlStatements();
  Integer rows=Limits.getDmlRows();
  String errorType=null;
  String errorMessage=null;
  Integer results=-1;
  List<Object> resultDetails=new List<Object>();
  try {
   List<Database.SaveResult> saved;
   AccessLevel mode=userMode?AccessLevel.USER_MODE:AccessLevel.SYSTEM_MODE;
   if(shape=='concrete') {List<SObject> records=new List<Account>{draft};saved=Database.insertAsync(records,new AsyncValidationCallback(),mode);}
   else saved=Database.insertAsync(new List<SObject>{draft},new AsyncValidationCallback(),mode);
   results=saved.size();
   for(Database.SaveResult entry:saved) {
    List<Object> errors=new List<Object>();
    for(Database.Error issue:entry.getErrors()) errors.add(new Map<String,Object>{'status'=>String.valueOf(issue.getStatusCode()),'message'=>issue.getMessage(),'fields'=>issue.getFields()});
    resultDetails.add(new Map<String,Object>{'success'=>entry.isSuccess(),'hasId'=>entry.getId()!=null,'errors'=>errors});
   }
  } catch(Exception issue) {errorType=issue.getTypeName();errorMessage=issue.getMessage();}
  Map<String,Object> result=new Map<String,Object>{'exceptionType'=>errorType,'exceptionMessage'=>errorMessage,'results'=>results,'resultDetails'=>resultDetails,'hasId'=>draft.Id!=null,'statements'=>Limits.getDmlStatements()-statements,'rows'=>Limits.getDmlRows()-rows,'records'=>[SELECT COUNT() FROM Account WHERE Name='Async validation'],'callbacksBefore'=>AsyncValidationCallback.calls};
  if(bounded) Test.stopTest();
  result.put('callbacksAfter',AsyncValidationCallback.calls);
  return result;
 }
@IsTest static void generic_user() {System.assert(false,'OBSERVED|generic_user|'+JSON.serialize(observe('generic',true)));}
@IsTest static void generic_system() {System.assert(false,'OBSERVED|generic_system|'+JSON.serialize(observe('generic',false)));}
@IsTest static void concrete_user() {System.assert(false,'OBSERVED|concrete_user|'+JSON.serialize(observe('concrete',true)));}
@IsTest static void concrete_system() {System.assert(false,'OBSERVED|concrete_system|'+JSON.serialize(observe('concrete',false)));}
@IsTest static void outside_user() {System.assert(false,'OBSERVED|outside_user|'+JSON.serialize(observe('outside',true)));}
@IsTest static void outside_system() {System.assert(false,'OBSERVED|outside_system|'+JSON.serialize(observe('outside',false)));}
}
