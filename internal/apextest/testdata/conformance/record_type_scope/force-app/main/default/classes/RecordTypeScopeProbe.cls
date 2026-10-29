public class RecordTypeScopeProbe {
 private static Object outcome(Database.SaveResult result) {
  if(result.isSuccess()) return 'success';
  Database.Error error=result.getErrors()[0];
  return new Map<String,Object>{'status'=>String.valueOf(error.getStatusCode()),'fields'=>error.getFields()};
 }
 public static Map<String,Object> observe() {
  Id foreignType=Account.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('PersonAccount').getRecordTypeId();
  Id localType=Ledger__c.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Local').getRecordTypeId();
  Map<String,Object> r=new Map<String,Object>();
  r.put('foreignInsert',outcome(Database.insert(new Ledger__c(Name='Foreign',RecordTypeId=foreignType),false)));
  Ledger__c local=new Ledger__c(Name='Local',RecordTypeId=localType);
  r.put('localInsert',outcome(Database.insert(local,false)));
  local.RecordTypeId=foreignType;
  r.put('foreignUpdate',outcome(Database.update(local,false)));
  r.put('defaultInsert',outcome(Database.insert(new Ledger__c(Name='Default'),false)));
  return r;
 }
}
