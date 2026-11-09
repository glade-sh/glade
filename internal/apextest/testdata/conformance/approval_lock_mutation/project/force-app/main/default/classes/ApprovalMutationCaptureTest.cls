@IsTest private class ApprovalMutationCaptureTest {
 @IsTest static void captureAdmin() { System.assert(false,'MUTATE_ROWS|'+ApprovalMutationCapture.observe()); }
 @IsTest static void captureStandard() {
  Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  User u=new User(Alias='probe',Email='probe@example.invalid',EmailEncodingKey='UTF-8',LastName='Probe',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',Username='approval-probe@example.invalid'); insert u;
  System.runAs(u) { System.assert(false,'MUTATE_ROWS|'+ApprovalMutationCapture.observe()); }
 }
}
