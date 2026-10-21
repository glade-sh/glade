@IsTest private class LookupOverflowRegression {
 private static Account seed(){
  Contact target=new Contact(LastName='Owned parent');insert target;
  Account owner=new Account(Name='Owned lookup owner',AnnualRevenue=12);
  for(Schema.SObjectField field:new List<Schema.SObjectField>{Account.AlphaBridge__c,Account.BetaBridge__c,Account.GammaBridge__c,Account.ZetaBridge__c})owner.put(field,target.Id);
  insert owner;return owner;
 }
 @IsTest static void explicitOverflowProjection(){
  Account owner=seed();Id rid=owner.Id;
  Account main=[SELECT Id,AnnualRevenue,AlphaBridge__c,AlphaBridge__r.Name FROM Account WHERE Id=:rid];
  Account extra=[SELECT Id,ZetaBridge__c,ZetaBridge__r.Name FROM Account WHERE Id=:rid];
  Boolean rejected=false;try{Object v=main.get(Account.ZetaBridge__c);}catch(SObjectException e){rejected=true;}System.assert(rejected);
  main.AnnualRevenue+=1;
  rejected=false;try{Object v=main.get('ZetaBridge__c');}catch(SObjectException e){rejected=true;}System.assert(rejected);
  System.assertEquals('Owned parent',extra.ZetaBridge__r.Name);
  main.put(Account.ZetaBridge__c,extra.get(Account.ZetaBridge__c));
  String relation=Account.ZetaBridge__c.getDescribe().getRelationshipName();main.putSObject(relation,extra.getSObject(relation));
  System.assertEquals(extra.ZetaBridge__c,main.ZetaBridge__c);System.assertEquals('Owned parent',main.getSObject(relation).get('Name'));
  rejected=false;try{Object v=main.get(Account.BetaBridge__c);}catch(SObjectException e){rejected=true;}System.assert(rejected);
 }
 @IsTest static void declaredTraversalCandidate(){
  Set<Schema.SObjectField> declared=new Set<Schema.SObjectField>{Account.OwnerId,Account.ParentId,Account.RecordTypeId,Account.AlphaBridge__c,Account.BetaBridge__c,Account.GammaBridge__c,Account.ZetaBridge__c};
  Map<String,Schema.SObjectField> fields=Account.SObjectType.getDescribe().fields.getMap();
  List<Schema.SObjectField> extra=new List<Schema.SObjectField>();Integer ordinal=0;
  for(String key:new List<String>(new Set<String>(fields.keySet()))){
   Schema.SObjectField token=fields.get(key);Schema.DescribeFieldResult facet=token.getDescribe();
   if(!declared.contains(token)||facet.getType()!=Schema.DisplayType.REFERENCE||!facet.isUpdateable())continue;
   if(ordinal>=4)extra.add(token);ordinal++;
  }
  System.assert(extra.contains(Account.ZetaBridge__c),'Candidate local-key policy puts Zeta beyond four slots');
 }
 private static void checkLimit(Integer slots){
  List<Schema.SObjectField> fields=new List<Schema.SObjectField>{Account.AlphaBridge__c,Account.BetaBridge__c,Account.GammaBridge__c,Account.ZetaBridge__c};
  Set<Schema.SObjectField> main=new Set<Schema.SObjectField>();Set<Schema.SObjectField> extra=new Set<Schema.SObjectField>();
  for(Integer i=0;i<fields.size();i++){if(i<slots)main.add(fields[i]);else extra.add(fields[i]);}
  System.assertEquals(4,main.size()+extra.size());System.assertEquals(slots<4,extra.contains(Account.ZetaBridge__c));
 }
 @IsTest static void zeroCapacity(){checkLimit(0);}
 @IsTest static void threeSlots(){checkLimit(3);}
 @IsTest static void ampleCapacity(){checkLimit(8);}
 @IsTest static void ordinaryMapControl(){
  Map<String,Integer> values=new Map<String,Integer>();values.put('Zeta',1);values.put('Alpha',2);
  System.assertEquals(new List<String>{'Zeta','Alpha'},new List<String>(values.keySet()));
 }
}
