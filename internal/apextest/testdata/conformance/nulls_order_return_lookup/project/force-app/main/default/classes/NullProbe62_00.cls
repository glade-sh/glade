@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|Partner|'+NullCore62.probeObject('Partner',new List<String>{'AccountFromId','OpportunityId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|ReturnOrder|'+NullCore62.probeObject('ReturnOrder',new List<String>{'OrderId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|ReturnOrderLineItem|'+NullCore62.probeObject('ReturnOrderLineItem',new List<String>{'AssetId','DestinationLocationId','OrderItemId','Product2Id','ProductItemId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|ReturnOrderLineItem|'+NullCore62.probeObject('ReturnOrderLineItem',new List<String>{'ProductRequestLineItemId','ProductServiceCampaignItemId','SourceLocationId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|WorkOrder|'+NullCore62.probeObject('WorkOrder',new List<String>{'AssetId','AssetWarrantyId','BusinessHoursId','MaintenancePlanId','ProductServiceCampaignItemId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|WorkOrder|'+NullCore62.probeObject('WorkOrder',new List<String>{'ReturnOrderLineItemId','ServiceReportTemplateId','ServiceTerritoryId'})); }
 @IsTest static void m06(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore62.probeObject('WorkOrderLineItem',new List<String>{'AssetId','AssetWarrantyId','MaintenancePlanId','OrderId','ParentWorkOrderLineItemId'})); }
 @IsTest static void m07(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore62.probeObject('WorkOrderLineItem',new List<String>{'PricebookEntryId','Product2Id','ProductServiceCampaignItemId','ReturnOrderLineItemId','ServiceReportTemplateId'})); }
 @IsTest static void m08(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore62.probeObject('WorkOrderLineItem',new List<String>{'ServiceTerritoryId'})); }
}
