public class ResidualRun {
 static Map<String,Object> res(Database.SaveResult r){
  Map<String,Object> m=new Map<String,Object>{'ok'=>r.isSuccess()};
  if(!r.isSuccess()){ Database.Error e=r.getErrors()[0]; m.put('code',String.valueOf(e.getStatusCode())); m.put('msg',e.getMessage()); m.put('dup',e instanceof Database.DuplicateError); }
  return m;
 }
 static void seedDup(){ insert new Lead(LastName='Seed',Company='Seed',Email='dup@probe.test',Phone='5551'); }
 static Id q;
 static Id queueId(){ if(q==null) q=[SELECT Id FROM Group WHERE Type='Queue' AND DeveloperName='ResidualQueue'].Id; return q; }
 static Database.DMLOptions opts(Boolean assign, Boolean allowSave){
  Database.DMLOptions o=new Database.DMLOptions();
  if(assign) o.assignmentRuleHeader.useDefaultRule=true;
  o.duplicateRuleHeader.allowSave=allowSave;
  return o;
 }
 static Map<String,Object> leadCase(Lead l, Boolean assign, Boolean allowSave){
  seedDup();
  Map<String,Object> m=res(Database.insert(l,opts(assign,allowSave)));
  m.put('rows',[SELECT count() FROM Lead]);
  return m;
 }
 public static String run(String n){
  Savepoint sp=Database.setSavepoint();
  Map<String,Object> m;
  try{ m=go(n); } finally { Database.rollback(sp); }
  return JSON.serialize(m);
 }
 static Map<String,Object> go(String n){
  switch on n {
   when 'dupBeforeFlow' { return leadCase(new Lead(LastName='B',Company='FlowBefore'),false,false); }
   when 'dupAfterFlow' { return leadCase(new Lead(LastName='B',Company='FlowAfter'),false,false); }
   when 'dupTrigger' { return leadCase(new Lead(LastName='B',Company='TrigDup'),false,false); }
   when 'dupAssignBlock' { return leadCase(new Lead(LastName='B',Company='RouteBlock',Email='dup@probe.test'),true,false); }
   when 'dupAssignBlockAllowSave' { return leadCase(new Lead(LastName='B',Company='RouteBlock',Email='dup@probe.test'),true,true); }
   when 'dupAssignAlert' { return leadCase(new Lead(LastName='B',Company='RouteAlert',Phone='5551'),true,false); }
   when 'dupAssignAlertAllowSave' {
    Lead l=new Lead(LastName='B',Company='RouteAlert',Phone='5551');
    Map<String,Object> m=leadCase(l,true,true);
    if(l.Id!=null) m.put('queueOwner',[SELECT OwnerId FROM Lead WHERE Id=:l.Id].OwnerId==queueId());
    return m;
   }
   when 'dupFlowAlert' { return leadCase(new Lead(LastName='B',Company='FlowAlert'),false,false); }
   when 'dupFlowAlertAllowSave' { return leadCase(new Lead(LastName='B',Company='FlowAlert'),false,true); }
   when 'dupFlowUpdate' { return leadCase(new Lead(LastName='B',Company='FlowUpd'),false,false); }
   when 'dupFlowCreateAllowSave' { return leadCase(new Lead(LastName='B',Company='FlowAfter'),false,true); }
   when 'dupApexUpdate' {
    seedDup(); Lead l=new Lead(LastName='B',Company='U'); insert l; l.Email='dup@probe.test';
    return res(Database.update(l,opts(false,false)));
   }
   when 'flowApexThrow' {
    Map<String,Object> m=new Map<String,Object>();
    try{ Database.insert(new Parcel__c(Name='a',Tag__c='throw'),AccessLevel.SYSTEM_MODE); m.put('thrown',false); }catch(DmlException e){ m.put('thrown',true); m.put('msg',e.getMessage().left(500)); }
    return m;
   }
   when 'flowApexThrowSave' { return res(Database.insert(new Parcel__c(Name='a',Tag__c='throw'),false,AccessLevel.SYSTEM_MODE)); }
   when 'flowApexThrowCaught' {
    Map<String,Object> m=new Map<String,Object>();
    try{ Database.insert(new Parcel__c(Name='a',Tag__c='throw'),AccessLevel.SYSTEM_MODE); }catch(DmlException e){ m.put('type',e.getTypeName()); m.put('code',String.valueOf(e.getDmlType(0))); m.put('n',e.getNumDml()); m.put('fields',e.getDmlFieldNames(0)); m.put('rows',[SELECT count() FROM Parcel__c WITH SYSTEM_MODE]); }
    return m;
   }
   when 'criteria' { return ResidualCriteria.run(); }
   when else { throw new IllegalArgumentException(n); }
  }
 }
}
