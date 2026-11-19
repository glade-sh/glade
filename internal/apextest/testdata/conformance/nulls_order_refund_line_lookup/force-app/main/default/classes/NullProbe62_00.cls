@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|PaymentLineInvoice|'+NullCore62.probeObject('PaymentLineInvoice',new List<String>{'AssociatedAccountId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|RefundLinePayment|'+NullCore62.probeObject('RefundLinePayment',new List<String>{'AssociatedAccountId'})); }
}
