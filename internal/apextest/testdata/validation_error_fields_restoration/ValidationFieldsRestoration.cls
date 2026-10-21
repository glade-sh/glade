@IsTest private class ValidationFieldsRestoration {
 // Restore error-field identity agreement; native namespace parity remains uncaptured.
 @IsTest static void insertFieldUsesDescribeIdentity() {
  Database.Error error=ValidationFieldsObserver.reject(false,'field');
  System.assertEquals(new List<String>{ValidationScoreLedger__c.Score__c.getDescribe().getName()},error.getFields());
 }
 @IsTest static void updateFieldUsesDescribeIdentity() {
  Database.Error error=ValidationFieldsObserver.reject(true,'field');
  System.assertEquals(new List<String>{ValidationScoreLedger__c.Score__c.getDescribe().getName()},error.getFields());
 }
 @IsTest static void recordLevelRetainsEmptyFields() {
  Database.Error error=ValidationFieldsObserver.reject(true,'record');
  System.assertEquals(new List<String>(),error.getFields());
  System.assertEquals('Negative score is unavailable.',error.getMessage());
 }
 @IsTest static void qualifiedMetadataRetainsIdentity() {
  Database.Error error=ValidationFieldsObserver.reject(true,'qualified');
  System.assertEquals(new List<String>{ValidationScoreLedger__c.QualifiedScore__c.getDescribe().getName()},error.getFields());
 }
 @IsTest static void storedLongTextNormalizesBoundaryOnly() {
  // Native-backed boundary normalization affects storage, preserving the caller.
  Database.Error error=ValidationFieldsObserver.reject(true,'field');
  String caller=ValidationFieldsObserver.assemble(error);
  ValidationErrorNotebook__c log=new ValidationErrorNotebook__c(Name='Control',Transcript__c=caller);
  insert log;
  System.assertEquals(caller,log.Transcript__c);
  ValidationErrorNotebook__c stored=[SELECT Transcript__c FROM ValidationErrorNotebook__c WHERE Id=:log.Id];
  System.assertEquals(caller.substring(0,caller.length()-1),stored.Transcript__c);
  System.assertEquals(caller,log.Transcript__c);
  System.assertEquals(10,caller.charAt(caller.length()-1));
  String expected=error.getMessage()+' : '+String.valueOf(error.getStatusCode())+' '+new List<String>{ValidationScoreLedger__c.Score__c.getDescribe().getName()};
  System.assertEquals(expected,stored.Transcript__c);
 }
}
