@IsTest private class PfNeighbor67 {
  public class Job implements Queueable {
    public void execute(QueueableContext c) { PfNeighbor67.seen = 'cr=' + Pf_Fld__c.Secret__c.getDescribe().isCreateable(); }
  }
  static String seen = 'none';
  static void fin(String s) { System.assert(false, 'RESULT => ' + s); }
  static void asg() { insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = 'Pf_Field'].Id); }
  // Uncaptured: an asynchronous job is its own transaction; it keeps the grant.
  @IsTest static void n1_queueable() { asg(); Test.startTest(); System.enqueueJob(new Job()); Test.stopTest(); fin(seen); }
  // Uncaptured: runAs with a Version keeps the user and the grant.
  @IsTest static void n2_runas_version() { asg(); String s; System.runAs(new Version(1, 0)) { s = 'cr=' + Pf_Fld__c.Secret__c.getDescribe().isCreateable(); } fin(s); }
  // Captured shape for reference (permset_in_test v08): another user keeps the grant.
  @IsTest static void n3_runas_user_assign_other() {
    Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
    User u = new User(Alias = 'pfn', Email = 'pf@example.invalid', EmailEncodingKey = 'UTF-8', LastName = 'PfN', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', UserName = 'pfn' + Math.abs(Crypto.getRandomInteger()) + '@example.invalid.glade');
    System.runAs(new User(Id = UserInfo.getUserId())) { insert u; insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = 'Pf_Field'].Id); }
    String s; System.runAs(u) { s = 'cr=' + Pf_Fld__c.Secret__c.getDescribe().isCreateable(); } fin(s);
  }
}
