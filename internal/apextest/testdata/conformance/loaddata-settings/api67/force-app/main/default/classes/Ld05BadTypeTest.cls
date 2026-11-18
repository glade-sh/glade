@IsTest private class Ld05BadTypeTest {
 @IsTest static void observe() {
  Map<String,Object> o=new Map<String,Object>();
  try { List<SObject> rows=Test.loadData(LoadRow__c.SObjectType,'LdBadType'); o.put('count',rows.size()); }
  catch(Exception e) { o.put('error',LdProbe.error(e)); }
  o.put('stored',[SELECT count() FROM LoadRow__c]);
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
