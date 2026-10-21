global class ValidationFieldsObserver {
 global static Database.Error reject(Boolean updating, String shape) {
  ValidationScoreLedger__c row = new ValidationScoreLedger__c(Name='Sample', Score__c=1, QualifiedScore__c=1, RecordOnly__c=shape=='record');
  if (updating) insert row;
  if (shape=='qualified') row.QualifiedScore__c=-1;
  else row.Score__c=-1;
  Database.SaveResult result;
  if (updating) result=Database.update(row,false);
  else result=Database.insert(row,false);
  System.assertEquals(false,result.isSuccess(),'fixture must exercise validation');
  System.assertEquals(1,result.getErrors().size(),'one error producer');
  return result.getErrors()[0];
 }
 private class ErrorHolder {
  private Database.Error held;
  ErrorHolder(Database.Error error) { held=error; }
  List<String> getFields() { return held.getFields(); }
  String getMessage() { return held.getMessage(); }
  StatusCode getStatusCode() { return held.getStatusCode(); }
 }
 public static String assemble(Database.Error error) {
  ErrorHolder holder=new ErrorHolder(error);
  List<String> fields=holder.getFields();
  return holder.getMessage()+' : '+String.valueOf(holder.getStatusCode())+(fields.isEmpty()?'':' '+fields)+'\n';
 }
}
