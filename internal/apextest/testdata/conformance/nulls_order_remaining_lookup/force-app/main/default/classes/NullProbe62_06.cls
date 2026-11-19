@IsTest(SeeAllData=false) private class NullProbe62_06 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|WorkCapacityLimit|'+NullCore62.probeObject('WorkCapacityLimit',new List<String>{'ServiceTerritoryId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|WorkCapacityUsage|'+NullCore62.probeObject('WorkCapacityUsage',new List<String>{'ServiceTerritoryId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|WorkOrder|'+NullCore62.probeObject('WorkOrder',new List<String>{'BusinessHoursId','ProductServiceCampaignItemId','ServiceReportTemplateId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore62.probeObject('WorkOrderLineItem',new List<String>{'Product2Id','ProductServiceCampaignItemId','ServiceReportTemplateId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|WorkPlan|'+NullCore62.probeObject('WorkPlan',new List<String>{'WorkOrderId','WorkOrderLineItemId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|WorkStep|'+NullCore62.probeObject('WorkStep',new List<String>{'PausedFlowInterviewId'})); }
 @IsTest static void m06(){ System.assert(false,'CAPTURE|WorkType|'+NullCore62.probeObject('WorkType',new List<String>{'ServiceReportTemplateId'})); }
}
