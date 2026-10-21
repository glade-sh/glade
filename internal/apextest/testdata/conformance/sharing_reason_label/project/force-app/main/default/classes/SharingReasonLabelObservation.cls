public class SharingReasonLabelObservation {
 public static List<Object> observe() {
  String customValue,customLabel,manualValue,manualLabel; Boolean customActive;
  for(Schema.PicklistEntry entry:Schema.ArchiveItem__Share.RowCause.getDescribe().getPicklistValues()) {
   if(entry.getValue()=='Authored_Audit__c') {customValue=entry.getValue();customLabel=entry.getLabel();customActive=entry.isActive();}
   if(entry.getValue()=='Manual') {manualValue=entry.getValue();manualLabel=entry.getLabel();}
  }
  return new List<Object>{customValue,customLabel,customActive,String.valueOf(Schema.ArchiveItem__Share.RowCause.Authored_Audit__c),manualValue,manualLabel};
 }
}
