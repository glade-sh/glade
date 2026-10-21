public with sharing class LegacyOmissionWrite {
 public static void apply(ProjectionTile__c item,String row){
  if(row=='A02L'||row=='SELECTED_L')item.Amount__c+=3;
  else if(row=='A03L')item.Amount__c-=3;
  else if(row=='A10L')item.ParentAccount__c=null;
 }
}
