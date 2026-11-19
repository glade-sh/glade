@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|DigitalWallet|'+NullCore62.probeObject('DigitalWallet',new List<String>{'PaymentGatewayId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|Payment|'+NullCore62.probeObject('Payment',new List<String>{'PaymentMethodId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|PaymentAuthorization|'+NullCore62.probeObject('PaymentAuthorization',new List<String>{'PaymentMethodId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|PaymentGateway|'+NullCore62.probeObject('PaymentGateway',new List<String>{'MerchantCredentialId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|PaymentGatewayLog|'+NullCore62.probeObject('PaymentGatewayLog',new List<String>{'PaymentGatewayId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|Refund|'+NullCore62.probeObject('Refund',new List<String>{'PaymentMethodId'})); }
}
