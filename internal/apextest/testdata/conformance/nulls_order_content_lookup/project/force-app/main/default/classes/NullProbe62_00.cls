@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|ContentVersion|'+NullCore62.probeObject('ContentVersion',new List<String>{'ContentBodyId','ExternalDataSourceId','FirstPublishLocationId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|EmailMessage|'+NullCore62.probeObject('EmailMessage',new List<String>{'ActivityId','EmailTemplateId','FromId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|FeedComment|'+NullCore62.probeObject('FeedComment',new List<String>{'LastEditById','RelatedRecordId','ThreadParentId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|FeedItem|'+NullCore62.probeObject('FeedItem',new List<String>{'LastEditById','RelatedRecordId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|ProductConsumed|'+NullCore62.probeObject('ProductConsumed',new List<String>{'PricebookEntryId','ProductItemId','WorkOrderLineItemId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|ServiceAppointment|'+NullCore62.probeObject('ServiceAppointment',new List<String>{'BundlePolicyId','ContactId','ParentRecordId','RelatedBundleId','ServiceTerritoryId'})); }
}
