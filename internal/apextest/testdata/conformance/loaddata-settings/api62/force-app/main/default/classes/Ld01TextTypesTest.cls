@IsTest private class Ld01TextTypesTest {
 @IsTest static void observe() {
  List<SObject> rows=Test.loadData(LoadRow__c.SObjectType,'LdTypes');
  LoadRow__c r=LdProbe.one('T1');
  Map<String,Object> o=new Map<String,Object>{'count'=>rows.size(),'text'=>r.Body__c,'num'=>String.valueOf(r.Num__c),
   'int'=>String.valueOf(r.Int__c),'flag'=>r.Flag__c,'day'=>String.valueOf(r.Day__c),'stamp'=>String.valueOf(r.Stamp__c),
   'retText'=>rows[0].get('Body__c'),'retNum'=>String.valueOf(rows[0].get('Num__c'))};
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
