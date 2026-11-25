@IsTest private class Road544Probe67 {
@IsTest static void IntegerAuthorizationRequest() { Integer amount = 3; String ref = 'ref'; commercepayments.AuthorizationRequest req = new commercepayments.AuthorizationRequest(amount); }
@IsTest static void IntegerCaptureRequest() { Integer amount = 3; String ref = 'ref'; commercepayments.CaptureRequest req = new commercepayments.CaptureRequest(amount, ref); }
@IsTest static void IntegerSaleRequest() { Integer amount = 3; String ref = 'ref'; commercepayments.SaleRequest req = new commercepayments.SaleRequest(amount); }
@IsTest static void IntegerReferencedRefundRequest() { Integer amount = 3; String ref = 'ref'; commercepayments.ReferencedRefundRequest req = new commercepayments.ReferencedRefundRequest(amount, ref); }
@IsTest static void IntegerAuthorizationReversalRequest() { Integer amount = 3; String ref = 'ref'; commercepayments.AuthorizationReversalRequest req = new commercepayments.AuthorizationReversalRequest(amount, ref); }
}
