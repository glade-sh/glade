@IsTest(SeeAllData=false) private class NullProbe67_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|ContractLineItem|'+NullCore67.probeObject('ContractLineItem',new List<String>{'AssetId','LocationId','ParentContractLineItemId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|Entitlement|'+NullCore67.probeObject('Entitlement',new List<String>{'AssetId','AssetWarrantyId','BusinessHoursId','ContractLineItemId','SlaProcessId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|OpportunityLineItem|'+NullCore67.probeObject('OpportunityLineItem',new List<String>{'PricebookEntryId','Product2Id'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|Payment|'+NullCore67.probeObject('Payment',new List<String>{'AccountId','PaymentAuthorizationId','PaymentGatewayId','PaymentGroupId','PaymentMethodId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|PaymentAuthorization|'+NullCore67.probeObject('PaymentAuthorization',new List<String>{'AccountId','PaymentGatewayId','PaymentGroupId','PaymentMethodId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|ProductConsumed|'+NullCore67.probeObject('ProductConsumed',new List<String>{'PricebookEntryId','ProductItemId','WorkOrderLineItemId'})); }
 @IsTest static void m06(){ System.assert(false,'CAPTURE|Refund|'+NullCore67.probeObject('Refund',new List<String>{'AccountId','PaymentGatewayId','PaymentGroupId','PaymentMethodId'})); }
}
