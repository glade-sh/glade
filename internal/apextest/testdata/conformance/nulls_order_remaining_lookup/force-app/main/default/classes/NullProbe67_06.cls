@IsTest(SeeAllData=false) private class NullProbe67_06 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|WorkCapacityLimit|'+NullCore67.probeObject('WorkCapacityLimit',new List<String>{'ServiceTerritoryId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|WorkCapacityUsage|'+NullCore67.probeObject('WorkCapacityUsage',new List<String>{'ServiceTerritoryId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|WorkOrder|'+NullCore67.probeObject('WorkOrder',new List<String>{'BusinessHoursId','ProductServiceCampaignItemId','ServiceReportTemplateId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore67.probeObject('WorkOrderLineItem',new List<String>{'Product2Id','ProductServiceCampaignItemId','ServiceReportTemplateId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|WorkPlan|'+NullCore67.probeObject('WorkPlan',new List<String>{'WorkOrderId','WorkOrderLineItemId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|WorkStep|'+NullCore67.probeObject('WorkStep',new List<String>{'PausedFlowInterviewId'})); }
 @IsTest static void m06(){ System.assert(false,'CAPTURE|WorkType|'+NullCore67.probeObject('WorkType',new List<String>{'ServiceReportTemplateId'})); }
}
