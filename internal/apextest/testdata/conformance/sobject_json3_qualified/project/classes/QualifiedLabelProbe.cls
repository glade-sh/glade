public class QualifiedLabelProbe {
 public static Map<String,Map<String,String>> observe() {
  Savepoint sp=Database.setSavepoint();
  Account a=new Account(Name='Owned T',Industry='Technology'); Account b=new Account(Name='Owned B',Industry='Banking'); insert new List<Account>{a,b};
  Contact c=new Contact(LastName='Owned C',AccountId=a.Id); insert c;
  Opportunity o=new Opportunity(Name='Owned O',AccountId=a.Id,StageName='Prospecting',CloseDate=Date.newInstance(2030,1,2)); insert o;
  insert new OpportunityContactRole(OpportunityId=o.Id,ContactId=c.Id,Role='Decision Maker');
  insert new Task(Subject='Owned Task',Status='Not Started',Priority='Normal');
  insert new Order(Name='Owned Order',AccountId=a.Id,EffectiveDate=Date.newInstance(2030,1,2),Status='Draft');
  insert new Contract(AccountId=a.Id,StartDate=Date.newInstance(2030,1,2),ContractTerm=12,Status='Draft',Description='Owned Contract');
  Map<String,String> queries=new Map<String,String>{
   'qualifiedAccount'=>'SELECT toLabel(Account.Industry) result FROM Account WHERE Name=\'Owned T\' AND toLabel(Account.Industry)=\'Technologie\'',
   'qualifiedOpportunity'=>'SELECT toLabel(Opportunity.StageName) result FROM Opportunity WHERE Name=\'Owned O\' AND toLabel(Opportunity.StageName)=\'Prospection FR\'',
   'qualifiedTask'=>'SELECT toLabel(Task.Status) result FROM Task WHERE Subject=\'Owned Task\' AND toLabel(Task.Status)=\'Pas commence\'',
   'statusTask'=>'SELECT Subject FROM Task WHERE Subject=\'Owned Task\' AND toLabel(Status)=\'Pas commence\'',
   'statusOrder'=>'SELECT Name FROM Order WHERE Name=\'Owned Order\' AND toLabel(Status)=\'Brouillon\'',
   'statusContract'=>'SELECT Description FROM Contract WHERE toLabel(Status)=\'Contrat brouillon\'',
   'parentOpportunity'=>'SELECT toLabel(Opportunity.StageName) result FROM OpportunityContactRole WHERE Opportunity.Name=\'Owned O\'',
   'semiIndustry'=>'SELECT LastName FROM Contact WHERE LastName=\'Owned C\' AND AccountId IN (SELECT Id FROM Account WHERE toLabel(Industry)=\'Technologie\')',
   'semiStage'=>'SELECT Name FROM Account WHERE Name=\'Owned T\' AND Id IN (SELECT AccountId FROM Opportunity WHERE toLabel(StageName)=\'Prospection FR\')',
   'semiMissing'=>'SELECT Name FROM Account WHERE Name=\'Owned T\' AND Id IN (SELECT AccountId FROM Opportunity WHERE toLabel(StageName)=\'Never\')'
  };
  Map<String,Map<String,String>> rows=new Map<String,Map<String,String>>();
  for(String shape:queries.keySet()) { try { List<SObject> found=Database.query(queries.get(shape)); List<Object> result=new List<Object>(); for(SObject x:found) { if(shape=='parentOpportunity') result.add(x.getSObject('Opportunity').get('result')); else if(shape.startsWith('qualified')) result.add(x.get('result')); else if(shape=='statusTask') result.add(x.get('Subject')); else if(shape=='statusContract') result.add(x.get('Description')); else if(shape=='semiIndustry') result.add(x.get('LastName')); else result.add(x.get('Name')); } rows.put(shape,new Map<String,String>{'value'=>JSON.serialize(result)}); } catch(Exception e) { rows.put(shape,new Map<String,String>{'value'=>e.getTypeName()+': '+e.getMessage()}); } }
  Database.rollback(sp);return rows;
 }
 public static void emit() { Map<String,Map<String,String>> rows=observe();for(String k:rows.keySet()) System.debug('QUALIFIED_LABEL|'+k+'|'+JSON.serialize(rows.get(k))); }
}
