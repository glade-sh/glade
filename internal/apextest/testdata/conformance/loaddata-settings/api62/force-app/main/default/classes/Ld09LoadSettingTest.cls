@IsTest private class Ld09LoadSettingTest {
 @IsTest static void observe() {
  delete [SELECT Id FROM HierPref__c];
  Map<String,Object> o=new Map<String,Object>();
  try { List<SObject> rows=Test.loadData(HierPref__c.SObjectType,'LdSetting'); o.put('count',rows.size()); }
  catch(Exception e) { o.put('error',LdProbe.error(e)); }
  HierPref__c d=HierPref__c.getOrgDefaults();
  o.put('org',d==null?'null':d.Value__c+'/'+String.valueOf(d.Num__c)+'/'+String.valueOf(d.Flag__c));
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
