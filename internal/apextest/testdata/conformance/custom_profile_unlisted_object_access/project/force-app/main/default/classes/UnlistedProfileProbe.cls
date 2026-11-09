public class UnlistedProfileProbe {
    static String err(Exception e) { return e.getTypeName().replace('System.', '') + ':' + e.getMessage(); }
    public static Map<String,Object> observe() {
        Map<String,Object> o = new Map<String,Object>();
        for (String pn : new List<String>{ 'UnlistedPlatform', 'UnlistedFull', 'ListedPlatform' }) {
            Map<String,Object> r = new Map<String,Object>();
            List<Profile> ps = [SELECT Id FROM Profile WHERE Name = :pn];
            r.put('profileCount', ps.size());
            if (ps.isEmpty()) { o.put(pn, r); continue; }
            r.put('inNoAccountReadQuery', [SELECT COUNT() FROM PermissionSet WHERE IsOwnedByProfile = true AND Profile.Name = :pn
                AND Id NOT IN (SELECT ParentId FROM ObjectPermissions WHERE SObjectType = 'Account' AND PermissionsRead = true)]);
            List<String> rows = new List<String>();
            for (ObjectPermissions op : [SELECT SObjectType, PermissionsRead, PermissionsCreate, PermissionsEdit, PermissionsDelete FROM ObjectPermissions
                WHERE Parent.IsOwnedByProfile = true AND Parent.Profile.Name = :pn]) {
                rows.add(op.SObjectType + ':' + (op.PermissionsRead ? 'R' : '-') + (op.PermissionsCreate ? 'C' : '-') + (op.PermissionsEdit ? 'E' : '-') + (op.PermissionsDelete ? 'D' : '-'));
            }
            rows.sort();
            r.put('objectRows', rows);
            if (!Test.isRunningTest()) { o.put(pn, r); continue; }
            User u = new User(FirstName = 'unl', LastName = 'probe', Alias = 'unl', Email = 'unl@example.com',
                Username = 'unl' + String.valueOf(Math.abs(Crypto.getRandomLong())) + '@example.com', EmailEncodingKey = 'ISO-8859-1',
                LanguageLocaleKey = 'en_US', TimeZoneSidKey = 'America/Los_Angeles', LocaleSidKey = 'en_US', ProfileId = ps[0].Id, IsActive = true);
            try { insert u; } catch (Exception e) { r.put('insert', err(e)); o.put(pn, r); continue; }
            System.runAs(u) {
                r.put('accountAccessible', Account.SObjectType.getDescribe().isAccessible());
                r.put('accountCreateable', Account.SObjectType.getDescribe().isCreateable());
                r.put('contactAccessible', Contact.SObjectType.getDescribe().isAccessible());
                r.put('contactCreateable', Contact.SObjectType.getDescribe().isCreateable());
                r.put('probeAccessible', UnlistedProbe__c.SObjectType.getDescribe().isAccessible());
                r.put('probeCreateable', UnlistedProbe__c.SObjectType.getDescribe().isCreateable());
            }
            o.put(pn, r);
        }
        return o;
    }
}
