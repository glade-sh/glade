@IsTest private class Asy_A4 {
    static String r(Id recId) { return 'perm=' + FeatureManagement.checkPermission('PsrPerm'); }
    public class Q implements Queueable { public void execute(QueueableContext c) { Asy_A4.out = 'q:' + FeatureManagement.checkPermission('PsrPerm') + ':' + UserInfo.getUserId().equals(Asy_A4.who); } }
    @future public static void fut() { Asy_A4.out = 'f:' + FeatureManagement.checkPermission('PsrPerm'); }
    public static String out = 'none'; public static Id who;
    static void asg(Id uid) { insert new PermissionSetAssignment(AssigneeId = uid, PermissionSetId = PsrUtil.permSetId()); }
    static Id me() { return UserInfo.getUserId(); }
    @IsTest static void t() {
        asg(me()); System.runAs(new User(Id=me())) { String s = r(null); } Test.startTest(); System.enqueueJob(new Q()); Test.stopTest();
        System.assert(false, 'A4 ' + out + ' | direct=' + FeatureManagement.checkPermission('PsrPerm'));
    }
}
