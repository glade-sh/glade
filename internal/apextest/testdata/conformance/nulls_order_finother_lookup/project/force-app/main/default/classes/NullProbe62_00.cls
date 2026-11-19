@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|FinanceTransaction|'+NullCore62.probeObject('FinanceTransaction',new List<String>{'AccountId','DestinationEntityId','LegalEntityId','ParentReferenceEntityId','ReferenceEntityId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|FinanceTransaction|'+NullCore62.probeObject('FinanceTransaction',new List<String>{'SourceEntityId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|Payment|'+NullCore62.probeObject('Payment',new List<String>{'AccountId','PaymentAuthorizationId','PaymentGatewayId','PaymentGroupId','PaymentMethodId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|PaymentAuthorization|'+NullCore62.probeObject('PaymentAuthorization',new List<String>{'AccountId','PaymentGatewayId','PaymentGroupId','PaymentMethodId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|PaymentGroup|'+NullCore62.probeObject('PaymentGroup',new List<String>{'SourceObjectId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|Refund|'+NullCore62.probeObject('Refund',new List<String>{'AccountId','PaymentGatewayId','PaymentGroupId','PaymentMethodId'})); }
}
