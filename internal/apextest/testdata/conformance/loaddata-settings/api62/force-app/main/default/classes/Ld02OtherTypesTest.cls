@IsTest private class Ld02OtherTypesTest {
 @IsTest static void observe() {
  Test.loadData(LoadRow__c.SObjectType,'LdTypes');
  LoadRow__c r=LdProbe.one('T1');
  Map<String,Object> o=new Map<String,Object>{'mailOk'=>r.Mail__c=='mail@example.test','phone'=>r.Phone__c,'choice'=>r.Choice__c,
   'pct'=>String.valueOf(r.Pct__c),'money'=>String.valueOf(r.Money__c),'required'=>r.Required__c};
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
