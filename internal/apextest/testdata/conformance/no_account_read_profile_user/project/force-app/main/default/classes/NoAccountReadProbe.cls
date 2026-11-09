public class NoAccountReadProbe {
    static String err(Exception e) { return e.getTypeName().replace('System.', '') + ':' + e.getMessage(); }
    public static Map<String,Object> observe() {
        Map<String,Object> o = new Map<String,Object>();
        List<PermissionSet> sets = [SELECT Profile.Id, Profile.Name FROM PermissionSet
            WHERE IsOwnedByProfile = true AND Profile.UserType = 'Standard'
            AND Id NOT IN (SELECT ParentId FROM ObjectPermissions WHERE SObjectType = 'Account' AND PermissionsRead = true)];
        List<String> names = new List<String>();
        for (PermissionSet ps : sets) names.add(ps.Profile.Name);
        names.sort();
        o.put('profileCount', names.size());
        o.put('profileNames', names);
        Map<String,Object> users = new Map<String,Object>();
        for (PermissionSet ps : sets) {
            User u = new User(FirstName = 'noacc', LastName = 'probe', Alias = 'noacc', Email = 'noacc@example.com',
                Username = 'noacc' + String.valueOf(Math.abs(Crypto.getRandomLong())) + '@example.com', EmailEncodingKey = 'ISO-8859-1',
                LanguageLocaleKey = 'en_US', TimeZoneSidKey = 'America/Los_Angeles', LocaleSidKey = 'en_US', ProfileId = ps.Profile.Id, IsActive = true);
            Map<String,Object> r = new Map<String,Object>();
            try { insert u; r.put('insert', 'ok'); } catch (Exception e) { r.put('insert', err(e)); users.put(ps.Profile.Name, r); continue; }
            System.runAs(u) {
                r.put('accountAccessible', Account.SObjectType.getDescribe().isAccessible());
                r.put('accountCreateable', Account.SObjectType.getDescribe().isCreateable());
                r.put('nameAccessible', Account.SObjectType.fields.Name.getDescribe().isAccessible());
                r.put('contactCreateable', Contact.SObjectType.getDescribe().isCreateable());
                r.put('contactAccessible', Contact.SObjectType.getDescribe().isAccessible());
                try { Integer c = [SELECT COUNT() FROM Account]; r.put('count', 'ok'); } catch (Exception e) { r.put('count', err(e)); }
            }
            users.put(ps.Profile.Name, r);
        }
        o.put('users', users);
        return o;
    }
}
