@IsTest(SeeAllData=false) private class NullProbe62_06 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|PaymentGateway|'+NullCore62.probeObject('PaymentGateway',new List<String>{'PaymentGatewayProviderId','MerchantCredentialId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|PaymentGatewayLog|'+NullCore62.probeObject('PaymentGatewayLog',new List<String>{'ReferencedEntityId','PaymentGatewayId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|PaymentGatewayProvider|'+NullCore62.probeObject('PaymentGatewayProvider',new List<String>{'ApexAdapterId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|PaymentGroup|'+NullCore62.probeObject('PaymentGroup',new List<String>{'SourceObjectId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|PaymentLineInvoice|'+NullCore62.probeObject('PaymentLineInvoice',new List<String>{'AssociatedAccountId','AssociatedPaymentLineId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|PermissionSet|'+NullCore62.probeObject('PermissionSet',new List<String>{'LicenseId'})); }
 @IsTest static void m06(){ System.assert(false,'CAPTURE|PermissionSetAssignment|'+NullCore62.probeObject('PermissionSetAssignment',new List<String>{'PermissionSetId','PermissionSetGroupId'})); }
 @IsTest static void m07(){ System.assert(false,'CAPTURE|PrivacyRTBFRequest|'+NullCore62.probeObject('PrivacyRTBFRequest',new List<String>{'PolicyNameId'})); }
 @IsTest static void m08(){ System.assert(false,'CAPTURE|ProcessException|'+NullCore62.probeObject('ProcessException',new List<String>{'CaseId','AssetId'})); }
 @IsTest static void m09(){ System.assert(false,'CAPTURE|Product2|'+NullCore62.probeObject('Product2',new List<String>{'ExternalDataSourceId'})); }
 @IsTest static void m10(){ System.assert(false,'CAPTURE|ProductConsumed|'+NullCore62.probeObject('ProductConsumed',new List<String>{'WorkOrderLineItemId','ProductItemId','PricebookEntryId'})); }
 @IsTest static void m11(){ System.assert(false,'CAPTURE|ProductRequest|'+NullCore62.probeObject('ProductRequest',new List<String>{'WorkOrderId','WorkOrderLineItemId','DestinationLocationId','SourceLocationId','CaseId'})); }
 @IsTest static void m12(){ System.assert(false,'CAPTURE|ProductRequest|'+NullCore62.probeObject('ProductRequest',new List<String>{'AccountId'})); }
 @IsTest static void m13(){ System.assert(false,'CAPTURE|ProductRequestLineItem|'+NullCore62.probeObject('ProductRequestLineItem',new List<String>{'WorkOrderId','WorkOrderLineItemId','DestinationLocationId','SourceLocationId','CaseId'})); }
 @IsTest static void m14(){ System.assert(false,'CAPTURE|ProductRequestLineItem|'+NullCore62.probeObject('ProductRequestLineItem',new List<String>{'AccountId'})); }
 @IsTest static void m15(){ System.assert(false,'CAPTURE|ProductServiceCampaign|'+NullCore62.probeObject('ProductServiceCampaign',new List<String>{'Product2Id','WorkTypeId'})); }
 @IsTest static void m16(){ System.assert(false,'CAPTURE|ProductServiceCampaignItem|'+NullCore62.probeObject('ProductServiceCampaignItem',new List<String>{'AssetId','SerializedProductId','Product2Id'})); }
 @IsTest static void m17(){ System.assert(false,'CAPTURE|ProductTransfer|'+NullCore62.probeObject('ProductTransfer',new List<String>{'ProductRequestLineItemId','SourceProductItemId','Product2Id','SourceLocationId','DestinationLocationId'})); }
 @IsTest static void m18(){ System.assert(false,'CAPTURE|ProductTransfer|'+NullCore62.probeObject('ProductTransfer',new List<String>{'ReceivedById','ShipmentId','ReturnOrderLineItemId'})); }
 @IsTest static void m19(){ System.assert(false,'CAPTURE|ProductWarrantyTerm|'+NullCore62.probeObject('ProductWarrantyTerm',new List<String>{'CoveredProductId'})); }
}
