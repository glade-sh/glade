public with sharing class LegacyOmissionRead {
 public static Object lookup(ProjectionTile__c item,String mode){
  if(mode=='direct')return item.ParentAccount__c;
  if(mode=='string')return item.get('ParentAccount__c');
  return item.get(ProjectionTile__c.ParentAccount__c);
 }
}
