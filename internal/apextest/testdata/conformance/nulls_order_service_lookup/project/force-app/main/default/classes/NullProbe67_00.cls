@IsTest(SeeAllData=false) private class NullProbe67_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|AssignedResource|'+NullCore67.probeObject('AssignedResource',new List<String>{'ServiceCrewId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|ServiceResource|'+NullCore67.probeObject('ServiceResource',new List<String>{'AccountId','LocationId','PayGroupId','RelatedRecordId','SchedulingConstraintId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|ServiceResource|'+NullCore67.probeObject('ServiceResource',new List<String>{'ServiceCrewId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|Shift|'+NullCore67.probeObject('Shift',new List<String>{'JobProfileId','RecordsetFilterCriteriaId','ServiceResourceId','ServiceTerritoryId','ShiftTemplateId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|TimeSheet|'+NullCore67.probeObject('TimeSheet',new List<String>{'ServiceResourceId'})); }
}
