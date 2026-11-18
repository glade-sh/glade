@IsTest private class Ld03QuotedBlankTest {
 @IsTest static void observe() {
  List<SObject> rows=Test.loadData(LoadRow__c.SObjectType,'LdBlank');
  Map<String,Object> o=new Map<String,Object>{'count'=>rows.size()};
  for (String name: new List<String>{'B1','B2','B3'}) {
   LoadRow__c r=LdProbe.one(name);
   o.put(name,'text='+r.Body__c+'|num='+String.valueOf(r.Num__c)+'|int='+String.valueOf(r.Int__c)+'|flag='+r.Flag__c+'|day='+String.valueOf(r.Day__c)+'|choice='+r.Choice__c);
  }
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
