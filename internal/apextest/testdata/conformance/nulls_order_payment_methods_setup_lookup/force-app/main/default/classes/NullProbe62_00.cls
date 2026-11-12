@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|AlternativePaymentMethod|'+NullCore62.probeObject('AlternativePaymentMethod',new List<String>{'PaymentGatewayId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|CardPaymentMethod|'+NullCore62.probeObject('CardPaymentMethod',new List<String>{'PaymentGatewayId'})); }
}
