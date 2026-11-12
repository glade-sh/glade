@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|AssignedResource|'+NullCore62.probeObject('AssignedResource',new List<String>{'ServiceCrewId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|MaintenanceWorkRule|'+NullCore62.probeObject('MaintenanceWorkRule',new List<String>{'ParentMaintenancePlanId','ParentMaintenanceRecordId','RecordsetFilterCriteriaId','WorkTypeId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|ServiceAppointment|'+NullCore62.probeObject('ServiceAppointment',new List<String>{'BundlePolicyId','ContactId','ParentRecordId','RelatedBundleId','ServiceTerritoryId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|ServiceResource|'+NullCore62.probeObject('ServiceResource',new List<String>{'AccountId','LocationId','PayGroupId','RelatedRecordId','SchedulingConstraintId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|ServiceResource|'+NullCore62.probeObject('ServiceResource',new List<String>{'ServiceCrewId'})); }
}
