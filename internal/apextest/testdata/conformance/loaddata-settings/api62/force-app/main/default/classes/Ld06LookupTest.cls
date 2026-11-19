@IsTest private class Ld06LookupTest {
 @IsTest static void observe() {
  LoadParent__c parent=new LoadParent__c(Name='parent-one'); insert parent;
  Map<String,Object> o=new Map<String,Object>{'parentId'=>(parent.Id!=null)};
  try { List<SObject> rows=Test.loadData(LoadRow__c.SObjectType,'LdLookup'); o.put('count',rows.size()); }
  catch(Exception e) { o.put('error',LdProbe.error(e)); }
  o.put('stored',[SELECT count() FROM LoadRow__c]);
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
