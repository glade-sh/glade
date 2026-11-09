@IsTest
private class PsTest {
    @IsTest static void t() {
        List<PermissionSet> a = [SELECT Id, Name, NamespacePrefix, Label FROM PermissionSet WHERE Name = 'Pkg_User'];
        System.assertEquals(1, a.size(), 'byName');
        System.assertEquals('basens', a[0].NamespacePrefix);
        System.assertEquals('Pkg_User', a[0].Name);
        System.assertEquals(0, [SELECT Id FROM PermissionSet WHERE Name = 'basens__Pkg_User'].size());
        System.assertEquals(1, [SELECT Id FROM PermissionSet WHERE Name = 'Pkg_User' AND NamespacePrefix = 'basens'].size());
        System.assertEquals(0, [SELECT Id FROM PermissionSet WHERE Name = 'Pkg_User' AND NamespacePrefix = null].size());
        System.assertEquals(false, FeatureManagement.checkPermission('Pkg_Run'), 'before');
        insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = a[0].Id);
        System.assertEquals(1, [SELECT Id FROM PermissionSetAssignment WHERE PermissionSet.Name = 'Pkg_User'].size());
        System.assertEquals(true, FeatureManagement.checkPermission('Pkg_Run'), 'after');
    }
}
