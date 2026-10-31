public class MorePredicateProbe {
 public static Map<String,Map<String,String>> observe() {
  Savepoint sp=Database.setSavepoint(); Account a=new Account(Name='Owned Parent',Industry='Technology'); insert a;
  insert new List<Opportunity>{new Opportunity(Name='Owned P',AccountId=a.Id,StageName='Prospecting',CloseDate=Date.newInstance(2030,1,2)),new Opportunity(Name='Owned Q',AccountId=a.Id,StageName='Qualification',CloseDate=Date.newInstance(2030,1,2)),new Opportunity(Name='Owned N',AccountId=a.Id,StageName='Needs Analysis',CloseDate=Date.newInstance(2030,1,2))};
  Map<String,String> queries=new Map<String,String>{
   'stageProspecting'=>'SELECT Name FROM Opportunity WHERE Name LIKE \'Owned %\' AND toLabel(StageName) = \'Prospection FR\' ORDER BY Name',
   'stageQualification'=>'SELECT Name FROM Opportunity WHERE Name LIKE \'Owned %\' AND toLabel(StageName) = \'Qualification FR\' ORDER BY Name',
   'stageNeeds'=>'SELECT Name FROM Opportunity WHERE Name LIKE \'Owned %\' AND toLabel(StageName) = \'Analyse FR\' ORDER BY Name',
   'stageMaster'=>'SELECT Name FROM Opportunity WHERE Name LIKE \'Owned %\' AND toLabel(StageName) = \'Prospecting\' ORDER BY Name',
   'stageMissing'=>'SELECT Name FROM Opportunity WHERE Name LIKE \'Owned %\' AND toLabel(StageName) = \'Never\' ORDER BY Name',
   'childProspecting'=>'SELECT (SELECT Name FROM Opportunities WHERE toLabel(StageName) = \'Prospection FR\') FROM Account WHERE Name=\'Owned Parent\'',
   'childQualification'=>'SELECT (SELECT Name FROM Opportunities WHERE toLabel(StageName) = \'Qualification FR\') FROM Account WHERE Name=\'Owned Parent\'',
   'parentPredicate'=>'SELECT Name FROM Opportunity WHERE Name LIKE \'Owned %\' AND toLabel(Account.Industry) = \'Technologie\' ORDER BY Name'
  };
  Map<String,Map<String,String>> rows=new Map<String,Map<String,String>>();
  for(String shape:queries.keySet()) { try {List<SObject> found=Database.query(queries.get(shape));List<Object> result=new List<Object>(); for(SObject x:found) {if(shape.startsWith('child')) {for(SObject child:x.getSObjects('Opportunities')) result.add(child.get('Name'));} else result.add(x.get('Name'));} rows.put(shape,new Map<String,String>{'value'=>JSON.serialize(result)});} catch(Exception e) { rows.put(shape,new Map<String,String>{'value'=>e.getTypeName()+': '+e.getMessage()}); }}
  Database.rollback(sp); return rows;
 }
 public static void emit() { Map<String,Map<String,String>> rows=observe();for(String k:rows.keySet()) System.debug('MORE_PREDICATE|'+k+'|'+JSON.serialize(rows.get(k))); }
}
