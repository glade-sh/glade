@IsTest(SeeAllData=true) private class NullProbe62_04 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|Partner|'+NullExistingCore62.probeObject('Partner',new List<String>{'AccountFromId','OpportunityId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|Payment|'+NullExistingCore62.probeObject('Payment',new List<String>{'AccountId','PaymentAuthorizationId','PaymentGatewayId','PaymentGroupId','PaymentMethodId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|PaymentAuthAdjustment|'+NullExistingCore62.probeObject('PaymentAuthAdjustment',new List<String>{'AccountId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|PaymentAuthorization|'+NullExistingCore62.probeObject('PaymentAuthorization',new List<String>{'AccountId','PaymentGatewayId','PaymentGroupId','PaymentMethodId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|PaymentGateway|'+NullExistingCore62.probeObject('PaymentGateway',new List<String>{'MerchantCredentialId','PaymentGatewayProviderId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|PaymentGatewayLog|'+NullExistingCore62.probeObject('PaymentGatewayLog',new List<String>{'PaymentGatewayId','ReferencedEntityId'})); }
 @IsTest static void m06(){ System.assert(false,'CAPTURE|PaymentGatewayProvider|'+NullExistingCore62.probeObject('PaymentGatewayProvider',new List<String>{'ApexAdapterId'})); }
 @IsTest static void m07(){ System.assert(false,'CAPTURE|PaymentGroup|'+NullExistingCore62.probeObject('PaymentGroup',new List<String>{'SourceObjectId'})); }
 @IsTest static void m08(){ System.assert(false,'CAPTURE|PaymentLineInvoice|'+NullExistingCore62.probeObject('PaymentLineInvoice',new List<String>{'AssociatedAccountId','AssociatedPaymentLineId'})); }
 @IsTest static void m09(){ System.assert(false,'CAPTURE|PermissionSetAssignment|'+NullExistingCore62.probeObject('PermissionSetAssignment',new List<String>{'PermissionSetGroupId','PermissionSetId'})); }
 @IsTest static void m10(){ System.assert(false,'CAPTURE|PrivacyRTBFRequest|'+NullExistingCore62.probeObject('PrivacyRTBFRequest',new List<String>{'PolicyNameId'})); }
 @IsTest static void m11(){ System.assert(false,'CAPTURE|ProcessException|'+NullExistingCore62.probeObject('ProcessException',new List<String>{'AssetId'})); }
 @IsTest static void m12(){ System.assert(false,'CAPTURE|Product2|'+NullExistingCore62.probeObject('Product2',new List<String>{'ExternalDataSourceId'})); }
 @IsTest static void m13(){ System.assert(false,'CAPTURE|ProductConsumed|'+NullExistingCore62.probeObject('ProductConsumed',new List<String>{'PricebookEntryId','ProductItemId','WorkOrderLineItemId'})); }
 @IsTest static void m14(){ System.assert(false,'CAPTURE|ProductServiceCampaignItem|'+NullExistingCore62.probeObject('ProductServiceCampaignItem',new List<String>{'AssetId','Product2Id','SerializedProductId'})); }
 @IsTest static void m15(){ System.assert(false,'CAPTURE|ProductTransfer|'+NullExistingCore62.probeObject('ProductTransfer',new List<String>{'ReturnOrderLineItemId','SourceProductItemId'})); }
 @IsTest static void m16(){ System.assert(false,'CAPTURE|ProductWarrantyTerm|'+NullExistingCore62.probeObject('ProductWarrantyTerm',new List<String>{'CoveredProductId'})); }
 @IsTest static void m17(){ System.assert(false,'CAPTURE|PromptVersion|'+NullExistingCore62.probeObject('PromptVersion',new List<String>{'ExperienceContextId','ImageId','PublishedByUserId','TargetRecordTypeId'})); }
 @IsTest static void m18(){ System.assert(false,'CAPTURE|Recommendation|'+NullExistingCore62.probeObject('Recommendation',new List<String>{'ImageId'})); }
 @IsTest static void m19(){ System.assert(false,'CAPTURE|RecordAction|'+NullExistingCore62.probeObject('RecordAction',new List<String>{'FlowInterviewId'})); }
}
