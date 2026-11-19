@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|AlternativePaymentMethod|'+NullCore62.probeObject('AlternativePaymentMethod',new List<String>{'PaymentGatewayId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|CardPaymentMethod|'+NullCore62.probeObject('CardPaymentMethod',new List<String>{'PaymentGatewayId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|Payment|'+NullCore62.probeObject('Payment',new List<String>{'PaymentGatewayId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|PaymentAuthorization|'+NullCore62.probeObject('PaymentAuthorization',new List<String>{'PaymentGatewayId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|PaymentGateway|'+NullCore62.probeObject('PaymentGateway',new List<String>{'PaymentGatewayProviderId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|Refund|'+NullCore62.probeObject('Refund',new List<String>{'PaymentGatewayId'})); }
}
