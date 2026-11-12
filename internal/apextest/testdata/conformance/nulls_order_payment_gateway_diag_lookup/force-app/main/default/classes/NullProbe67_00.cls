@IsTest(SeeAllData=false) private class NullProbe67_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|AlternativePaymentMethod|'+NullCore67.probeObject('AlternativePaymentMethod',new List<String>{'PaymentGatewayId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|CardPaymentMethod|'+NullCore67.probeObject('CardPaymentMethod',new List<String>{'PaymentGatewayId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|Payment|'+NullCore67.probeObject('Payment',new List<String>{'PaymentGatewayId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|PaymentAuthorization|'+NullCore67.probeObject('PaymentAuthorization',new List<String>{'PaymentGatewayId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|PaymentGateway|'+NullCore67.probeObject('PaymentGateway',new List<String>{'PaymentGatewayProviderId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|Refund|'+NullCore67.probeObject('Refund',new List<String>{'PaymentGatewayId'})); }
}
