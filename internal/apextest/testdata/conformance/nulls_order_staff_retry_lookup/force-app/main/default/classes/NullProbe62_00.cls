@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|ServiceAppointment|'+NullCore62.probeObject('ServiceAppointment',new List<String>{'RelatedBundleId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|ServiceResource|'+NullCore62.probeObject('ServiceResource',new List<String>{'LocationId','PayGroupId','ServiceCrewId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|TimeSheetEntry|'+NullCore62.probeObject('TimeSheetEntry',new List<String>{'WorkOrderId','WorkOrderLineItemId'})); }
}
