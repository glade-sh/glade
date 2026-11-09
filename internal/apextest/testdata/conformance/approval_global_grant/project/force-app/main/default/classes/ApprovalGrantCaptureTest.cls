@IsTest(SeeAllData=true) private class ApprovalGrantCaptureTest {
 @IsTest static void capture() {
  User u=[SELECT Id FROM User WHERE Profile.Name='Standard User' AND IsActive=true LIMIT 1];
  System.runAs(u) {
   Account a=new Account(Name='Permission probe'); insert a; Approval.lock(a); a.Name='Changed';
   Database.SaveResult r=Database.update(a,false);
   System.assert(false,'PERMISSION_ROW|'+r.isSuccess()+':'+(r.isSuccess()?'NONE':String.valueOf(r.getErrors()[0].getStatusCode())));
  }
 }
}
