@IsTest private class Asy_A8 {
    static String r(Id recId) { return 'perm=' + FeatureManagement.checkPermission('PsrPerm'); }
    public class Q implements Queueable { public void execute(QueueableContext c) { Asy_A8.out = 'q:' + FeatureManagement.checkPermission('PsrPerm') + ':' + UserInfo.getUserId().equals(Asy_A8.who); } }
    @future public static void fut() { Asy_A8.out = 'f:' + FeatureManagement.checkPermission('PsrPerm'); }
    public static String out = 'none'; public static Id who;
    static void asg(Id uid) { insert new PermissionSetAssignment(AssigneeId = uid, PermissionSetId = PsrUtil.permSetId()); }
    static Id me() { return UserInfo.getUserId(); }
    @IsTest static void t() {
        asg(me()); Test.startTest(); fut(); Test.stopTest();
        System.assert(false, 'A8 ' + out + ' | direct=' + FeatureManagement.checkPermission('PsrPerm'));
    }
}
