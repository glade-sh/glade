@IsTest private class Asy_A5 {
    static String r(Id recId) { return 'perm=' + FeatureManagement.checkPermission('PsrPerm'); }
    public class Q implements Queueable { public void execute(QueueableContext c) { Asy_A5.out = 'q:' + FeatureManagement.checkPermission('PsrPerm') + ':' + UserInfo.getUserId().equals(Asy_A5.who); } }
    @future public static void fut() { Asy_A5.out = 'f:' + FeatureManagement.checkPermission('PsrPerm'); }
    public static String out = 'none'; public static Id who;
    static void asg(Id uid) { insert new PermissionSetAssignment(AssigneeId = uid, PermissionSetId = PsrUtil.permSetId()); }
    static Id me() { return UserInfo.getUserId(); }
    @IsTest static void t() {
        User u = PsrUtil.newUser(PsrUtil.limitedProfileId(),'AS',5); insert u; who = u.Id; asg(u.Id); Test.startTest(); System.runAs(u) { System.enqueueJob(new Q()); } Test.stopTest();
        System.assert(false, 'A5 ' + out + ' | direct=' + FeatureManagement.checkPermission('PsrPerm'));
    }
}
