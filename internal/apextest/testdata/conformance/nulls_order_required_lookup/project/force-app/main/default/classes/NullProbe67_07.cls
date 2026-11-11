@IsTest(SeeAllData=false) private class NullProbe67_07 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|WorkPlan|'+NullCore67.probeObject('WorkPlan',new List<String>{'WorkOrderId','WorkOrderLineItemId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|WorkPlanSelectionRule|'+NullCore67.probeObject('WorkPlanSelectionRule',new List<String>{'AssetId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|WorkStep|'+NullCore67.probeObject('WorkStep',new List<String>{'PausedFlowInterviewId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|WorkType|'+NullCore67.probeObject('WorkType',new List<String>{'ServiceReportTemplateId'})); }
}
