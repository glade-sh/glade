public class LegacyOmissionQuery {
 public static ProjectionTile__c fetch(Id rid,Boolean selected){
  String fields='Id,Amount__c,CountValue__c';if(selected)fields+=',ParentAccount__c';
  return (ProjectionTile__c)Database.query('SELECT '+fields+' FROM ProjectionTile__c WHERE Id=:rid');
 }
}
