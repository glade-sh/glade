public class SObjectJSON3BoundaryProbe {
 public static Map<String,Map<String,String>> observe() {
  Savepoint sp=Database.setSavepoint();
  Json_Number__c pos=new Json_Number__c(Name='Positive',Money__c=1234.5,Day__c=Date.newInstance(2030,1,2));
  Json_Number__c neg=new Json_Number__c(Name='Negative',Money__c=-12.34,Day__c=Date.newInstance(2028,6,15));
  Json_Number__c nul=new Json_Number__c(Name='Null'); insert new List<Json_Number__c>{pos,neg,nul};
  Account a=new Account(Name='Agriculture',Industry='Agriculture'); Account b=new Account(Name='Banking',Industry='Banking'); Account c=new Account(Name='Technology',Industry='Technology'); Account empty=new Account(Name='Empty'); insert new List<Account>{a,b,c,empty};
  Opportunity o1=new Opportunity(Name='One',StageName='Prospecting',CloseDate=Date.newInstance(2030,1,2)); Opportunity o2=new Opportunity(Name='Two',StageName='Qualification',CloseDate=Date.newInstance(2030,1,2)); Opportunity o3=new Opportunity(Name='Three',StageName='Needs Analysis',CloseDate=Date.newInstance(2030,1,2)); insert new List<Opportunity>{o1,o2,o3};
  Map<String,Id> ids=new Map<String,Id>{'formatMoney'=>pos.Id,'formatNegative'=>neg.Id,'formatNull'=>nul.Id,'formatDate'=>pos.Id,'formatDateNegative'=>neg.Id,'labelAgriculture'=>a.Id,'labelBanking'=>b.Id,'labelTechnology'=>c.Id,'labelNull'=>empty.Id,'stageProspecting'=>o1.Id,'stageQualification'=>o2.Id,'stageNeeds'=>o3.Id};
  Map<String,String> queries=new Map<String,String>{
   'formatMoney'=>'SELECT FORMAT(Money__c) result FROM Json_Number__c WHERE Id=:id',
   'formatNegative'=>'SELECT FORMAT(Money__c) result FROM Json_Number__c WHERE Id=:id',
   'formatNull'=>'SELECT FORMAT(Money__c) result FROM Json_Number__c WHERE Id=:id',
   'formatDate'=>'SELECT FORMAT(Day__c) result FROM Json_Number__c WHERE Id=:id',
   'formatDateNegative'=>'SELECT FORMAT(Day__c) result FROM Json_Number__c WHERE Id=:id',
   'labelAgriculture'=>'SELECT toLabel(Industry) result FROM Account WHERE Id=:id',
   'labelBanking'=>'SELECT toLabel(Industry) result FROM Account WHERE Id=:id',
   'labelTechnology'=>'SELECT toLabel(Industry) result FROM Account WHERE Id=:id',
   'labelNull'=>'SELECT toLabel(Industry) result FROM Account WHERE Id=:id',
   'stageProspecting'=>'SELECT toLabel(StageName) result FROM Opportunity WHERE Id=:id',
   'stageQualification'=>'SELECT toLabel(StageName) result FROM Opportunity WHERE Id=:id',
   'stageNeeds'=>'SELECT toLabel(StageName) result FROM Opportunity WHERE Id=:id'
  };
  Map<String,Map<String,String>> rows=new Map<String,Map<String,String>>();
  for(String shape:queries.keySet()) {
   try { Id id=ids.get(shape); SObject result=Database.query(queries.get(shape))[0]; rows.put(shape,new Map<String,String>{'value'=>JSON.serialize(result.get('result'))}); }
   catch(Exception e) {rows.put(shape,new Map<String,String>{'value'=>e.getTypeName()+': '+e.getMessage()});}
  }
  Database.rollback(sp); return rows;
 }
 public static void emit() {Map<String,Map<String,String>> rows=observe();for(String k:rows.keySet()) System.debug('SOBJECT_BOUNDARY|'+k+'|'+JSON.serialize(rows.get(k)));}
}
