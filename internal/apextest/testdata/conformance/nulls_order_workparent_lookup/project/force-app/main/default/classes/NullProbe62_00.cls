@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|ProductConsumed|'+NullCore62.probeObject('ProductConsumed',new List<String>{'PricebookEntryId','ProductItemId','WorkOrderLineItemId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore62.probeObject('WorkOrderLineItem',new List<String>{'AssetId','AssetWarrantyId','MaintenancePlanId','OrderId','ParentWorkOrderLineItemId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore62.probeObject('WorkOrderLineItem',new List<String>{'PricebookEntryId','Product2Id','ProductServiceCampaignItemId','ReturnOrderLineItemId','ServiceReportTemplateId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore62.probeObject('WorkOrderLineItem',new List<String>{'ServiceTerritoryId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|WorkPlan|'+NullCore62.probeObject('WorkPlan',new List<String>{'WorkOrderId','WorkOrderLineItemId'})); }
}
