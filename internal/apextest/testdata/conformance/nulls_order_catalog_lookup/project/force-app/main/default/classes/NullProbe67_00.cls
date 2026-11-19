@IsTest(SeeAllData=false) private class NullProbe67_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|ContractLineItem|'+NullCore67.probeObject('ContractLineItem',new List<String>{'AssetId','LocationId','ParentContractLineItemId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|Order|'+NullCore67.probeObject('Order',new List<String>{'AccountId','BillToContactId','CompanyAuthorizedById','CustomerAuthorizedById','OriginalOrderId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|OrderItem|'+NullCore67.probeObject('OrderItem',new List<String>{'OriginalOrderItemId','Product2Id'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|ProductConsumed|'+NullCore67.probeObject('ProductConsumed',new List<String>{'PricebookEntryId','ProductItemId','WorkOrderLineItemId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|ReturnOrderLineItem|'+NullCore67.probeObject('ReturnOrderLineItem',new List<String>{'AssetId','DestinationLocationId','OrderItemId','Product2Id','ProductItemId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|ReturnOrderLineItem|'+NullCore67.probeObject('ReturnOrderLineItem',new List<String>{'ProductRequestLineItemId','ProductServiceCampaignItemId','SourceLocationId'})); }
 @IsTest static void m06(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore67.probeObject('WorkOrderLineItem',new List<String>{'AssetId','AssetWarrantyId','MaintenancePlanId','OrderId','ParentWorkOrderLineItemId'})); }
 @IsTest static void m07(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore67.probeObject('WorkOrderLineItem',new List<String>{'PricebookEntryId','Product2Id','ProductServiceCampaignItemId','ReturnOrderLineItemId','ServiceReportTemplateId'})); }
 @IsTest static void m08(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore67.probeObject('WorkOrderLineItem',new List<String>{'ServiceTerritoryId'})); }
}
