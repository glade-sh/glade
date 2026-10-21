public with sharing class FamilyL09ExternalDml {
 @AuraEnabled
 public static void externalUpdate(Id recordId) {
  Account owned=[SELECT Id, Name FROM Account WHERE Id=:recordId LIMIT 1];
  if(!owned.Name.startsWith('L09_ORACLE_')) {
   throw new AuraHandledException('L09 fixture refuses unrelated record');
  }
  owned.Description='AFTER_EXTERNAL';
  update owned;
 }

 @AuraEnabled
 public static Id createUncached(String name) {
  if(name == null || !name.startsWith('L09_ORACLE_')) {
   throw new AuraHandledException('L09 fixture refuses unrelated record');
  }
  Account owned=new Account(Name=name, Description='OWNED_DESCRIPTION',
   AnnualRevenue=12.5, NumberOfEmployees=7);
  insert owned;
  return owned.Id;
 }
 @AuraEnabled
 public static Map<String, Object> queryContext() {
  Map<String, Schema.SObjectField> fields=Schema.SObjectType.Account.fields.getMap();
  List<String> order=new List<String>(fields.keySet());
  List<Object> descriptions=new List<Object>();
  for(String name : order) {
   Schema.DescribeFieldResult field=fields.get(name).getDescribe();
   List<String> references=new List<String>();
   for(Schema.SObjectType target : field.getReferenceTo()) {
    references.add(target.getDescribe().getName());
   }
   descriptions.add(new Map<String, Object>{'name'=>field.getName(),
    'type'=>String.valueOf(field.getType()), 'accessible'=>field.isAccessible(),
    'createable'=>field.isCreateable(), 'updateable'=>field.isUpdateable(),
    'relationshipName'=>field.getRelationshipName(), 'referenceTo'=>references});
  }
  return new Map<String, Object>{'fieldOrder'=>order, 'fields'=>descriptions};
 }
}