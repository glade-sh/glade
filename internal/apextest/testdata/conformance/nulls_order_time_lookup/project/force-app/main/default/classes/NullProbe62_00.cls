@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|ContentDistribution|'+NullCore62.probeObject('ContentDistribution',new List<String>{'RelatedRecordId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|ReturnOrderLineItem|'+NullCore62.probeObject('ReturnOrderLineItem',new List<String>{'AssetId','DestinationLocationId','OrderItemId','Product2Id','ProductItemId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|ReturnOrderLineItem|'+NullCore62.probeObject('ReturnOrderLineItem',new List<String>{'ProductRequestLineItemId','ProductServiceCampaignItemId','SourceLocationId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|Shift|'+NullCore62.probeObject('Shift',new List<String>{'JobProfileId','RecordsetFilterCriteriaId','ServiceResourceId','ServiceTerritoryId','ShiftTemplateId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|TimeSlot|'+NullCore62.probeObject('TimeSlot',new List<String>{'RecordsetFilterCriteriaId'})); }
}
