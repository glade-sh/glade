@IsTest private class PaymentIdentityTest {
 @IsTest static void capture_identity() {
  String authorization='a00000000000001';
  Double amount=7.25;
  commercepayments.CaptureRequest request=new commercepayments.CaptureRequest(amount,authorization);
  System.assert(false,'OBSERVED|capture_identity|'+JSON.serialize(new Map<String,Object>{'identityPreserved'=>request.paymentAuthorizationId==authorization,'amountPreserved'=>request.amount==7.25}));
 }
 @IsTest static void capture_null() {
  commercepayments.CaptureRequest request=new commercepayments.CaptureRequest(null,null);
  System.assert(false,'OBSERVED|capture_null|'+JSON.serialize(new Map<String,Object>{'identityNull'=>request.paymentAuthorizationId==null,'amountNull'=>request.amount==null}));
 }
 @IsTest static void reversal_identity() {
  String authorization='a00000000000001'; Double amount=7.25;
  commercepayments.AuthorizationReversalRequest request=new commercepayments.AuthorizationReversalRequest(amount,authorization);
  System.assert(false,'OBSERVED|reversal_identity|'+JSON.serialize(new Map<String,Object>{'identityPreserved'=>request.paymentAuthorizationId==authorization,'amountPreserved'=>request.amount==amount}));
 }
 @IsTest static void card_identity() {
  commercepayments.CardPaymentMethodRequest card=new commercepayments.CardPaymentMethodRequest(commercepayments.CardCategory.CreditCard);
  commercepayments.AuthApiPaymentMethodRequest request=new commercepayments.AuthApiPaymentMethodRequest(card);
  System.assert(false,'OBSERVED|card_identity|'+JSON.serialize(new Map<String,Object>{'identityPreserved'=>request.cardPaymentMethod===card}));
 }
 @IsTest static void notification_empty() {
  Map<String,Object> observed=new Map<String,Object>();
  try {
   commercepayments.NotificationSaveResult result=commercepayments.NotificationClient.record(new commercepayments.CaptureNotification());
   observed.put('status',result.getStatusCode());observed.put('success',result.isSuccess());observed.put('message',result.getErrorMessage());
  } catch(Exception e){observed.put('exception',e.getTypeName());observed.put('message',e.getMessage());}
  System.assert(false,'OBSERVED|notification_empty|'+JSON.serialize(observed));
 }
 @IsTest static void notification_null() {
  Map<String,Object> observed=new Map<String,Object>();
  try {commercepayments.NotificationClient.record(null);observed.put('returned',true);}
  catch(Exception e){observed.put('exception',e.getTypeName());observed.put('message',e.getMessage());}
  System.assert(false,'OBSERVED|notification_null|'+JSON.serialize(observed));
 }
 @IsTest static void notification_declined() {
  commercepayments.ReferencedRefundNotification notice=new commercepayments.ReferencedRefundNotification();
  Double amount=1.0; notice.setAmount(amount);notice.setStatus(commercepayments.NotificationStatus.Failed);
  notice.setSalesforceResultCodeInfo(new commercepayments.SalesforceResultCodeInfo(commercepayments.SalesforceResultCode.Decline));
  notice.setGatewayReferenceNumber('synthetic-reference');notice.setGatewayResultCode('synthetic-code');notice.setGatewayMessage('Synthetic failure');
  Map<String,Object> observed=new Map<String,Object>();
  try {
   commercepayments.NotificationSaveResult result=commercepayments.NotificationClient.record(notice);
   observed.put('status',result.getStatusCode());observed.put('success',result.isSuccess());observed.put('message',result.getErrorMessage());
  } catch(Exception e){observed.put('exception',e.getTypeName());observed.put('message',e.getMessage());}
  System.assert(false,'OBSERVED|notification_declined|'+JSON.serialize(observed));
 }
}
