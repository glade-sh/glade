public class GlGrant {
  public static void run() {
    PermissionSet ps = [SELECT Id FROM PermissionSet WHERE Name = 'GlProbeAll'];
    if ([SELECT COUNT() FROM PermissionSetAssignment WHERE PermissionSetId = :ps.Id AND AssigneeId = :UserInfo.getUserId()] == 0) {
      System.runAs(new User(Id = UserInfo.getUserId())) {
        insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = ps.Id);
      }
    }
  }
}
