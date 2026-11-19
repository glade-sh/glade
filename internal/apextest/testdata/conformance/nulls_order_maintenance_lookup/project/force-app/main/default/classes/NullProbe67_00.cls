@IsTest(SeeAllData=false) private class NullProbe67_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|MaintenanceWorkRule|'+NullCore67.probeObject('MaintenanceWorkRule',new List<String>{'ParentMaintenancePlanId','ParentMaintenanceRecordId','RecordsetFilterCriteriaId','WorkTypeId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|WorkOrder|'+NullCore67.probeObject('WorkOrder',new List<String>{'AssetId','AssetWarrantyId','BusinessHoursId','MaintenancePlanId','ProductServiceCampaignItemId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|WorkOrder|'+NullCore67.probeObject('WorkOrder',new List<String>{'ReturnOrderLineItemId','ServiceReportTemplateId','ServiceTerritoryId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore67.probeObject('WorkOrderLineItem',new List<String>{'AssetId','AssetWarrantyId','MaintenancePlanId','OrderId','ParentWorkOrderLineItemId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore67.probeObject('WorkOrderLineItem',new List<String>{'PricebookEntryId','Product2Id','ProductServiceCampaignItemId','ReturnOrderLineItemId','ServiceReportTemplateId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore67.probeObject('WorkOrderLineItem',new List<String>{'ServiceTerritoryId'})); }
}
