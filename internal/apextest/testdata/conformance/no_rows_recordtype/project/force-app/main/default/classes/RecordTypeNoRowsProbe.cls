public class RecordTypeNoRowsProbe {
 public static String sample() {
  Map<String,Object> out=new Map<String,Object>();
  out.put('existing',[SELECT COUNT() FROM RecordType WHERE SObjectType='TypeProbe__c' AND DeveloperName='Synthetic_Type']);
  out.put('missing',[SELECT COUNT() FROM RecordType WHERE SObjectType='TypeProbe__c' AND DeveloperName='Synthetic_Absent']);
  try { RecordType r=[SELECT Id FROM RecordType WHERE SObjectType='TypeProbe__c' AND DeveloperName='Synthetic_Absent']; out.put('singleton','found'); }
  catch(QueryException e) { out.put('singleton',e.getTypeName()); }
  return JSON.serialize(out);
 }
}
