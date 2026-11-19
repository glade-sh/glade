@IsTest(SeeAllData=false) private class NullProbe67_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|PaymentLineInvoice|'+NullCore67.probeObject('PaymentLineInvoice',new List<String>{'AssociatedAccountId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|RefundLinePayment|'+NullCore67.probeObject('RefundLinePayment',new List<String>{'AssociatedAccountId'})); }
}
