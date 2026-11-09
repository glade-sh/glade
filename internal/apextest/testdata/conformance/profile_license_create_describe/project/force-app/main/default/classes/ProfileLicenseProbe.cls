public class ProfileLicenseProbe {
    static String err(Exception e) { return e.getTypeName().replace('System.', '') + ':' + e.getMessage(); }
    public static Map<String,Object> observe() {
        Map<String,Object> o = new Map<String,Object>();
        Schema.DescribeSObjectResult d = Profile.SObjectType.getDescribe();
        o.put('objCreateable', d.isCreateable());
        o.put('objUpdateable', d.isUpdateable());
        Map<String,Schema.SObjectField> m = d.fields.getMap();
        for (String n : new List<String>{ 'UserLicenseId', 'Name', 'Description', 'PermissionsApiEnabled', 'UserType' }) {
            Schema.DescribeFieldResult f = m.get(n).getDescribe();
            o.put(n, new Map<String,Object>{ 'createable' => f.isCreateable(), 'updateable' => f.isUpdateable(), 'nillable' => f.isNillable(),
                'defaulted' => f.isDefaultedOnCreate(), 'accessible' => f.isAccessible() });
        }
        o.put('userLicenseObjCreateable', UserLicense.SObjectType.getDescribe().isCreateable());
        return o;
    }
}
