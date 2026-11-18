List<String> ns=new List<String>{'Account','GladeMissingProbe__c','AIFeatureExtractor','AIResearchPromptResult','AIScoringStatusEvent','AssociatedLocationHistory'};
List<String> tables=new List<String>{'FieldDefinition','EntityParticle','RelationshipDomain','UserEntityAccess','UserFieldAccess'};
List<String> keys=new List<String>{'EntityDefinitionId','EntityDefinitionId','ChildSobjectId','EntityDefinitionId','EntityDefinitionId'};
List<String> results=new List<String>();
results.add('AccountControl='+(Type.forName('Account')!=null));
results.add('MissingControl='+(Type.forName('GladeMissingProbe__c')!=null));
for(Integer i=0;i<tables.size();i++) {
 for(String n:ns) {
  String q='SELECT count() FROM '+tables[i]+' WHERE '+keys[i]+' = \''+String.escapeSingleQuotes(n)+'\'';
  String v;
  try { v=String.valueOf(Database.countQuery(q)>0); } catch(Exception e) { v=e.getTypeName(); }
  results.add(tables[i]+':'+n+'='+v);
 }
}
System.assert(false,'OBS|'+String.join(results,';'));
