public class UserPsaProbe {
    static Object assignments(Id userId) {
        List<String> rows = new List<String>();
        for (PermissionSetAssignment a : [SELECT PermissionSet.IsOwnedByProfile, PermissionSet.ProfileId, PermissionSet.Label, PermissionSet.Name FROM PermissionSetAssignment WHERE AssigneeId = :userId ORDER BY PermissionSet.IsOwnedByProfile]) {
            rows.add((a.PermissionSet.IsOwnedByProfile ? 'profile' : 'set') + ':' + (a.PermissionSet.ProfileId != null) + ':' + (a.PermissionSet.IsOwnedByProfile ? '' : a.PermissionSet.Name));
        }
        return rows;
    }
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        m.put('running', assignments(UserInfo.getUserId()));
        if (!Test.isRunningTest()) { m.put('created', 'not a test run'); return m; }
        for (String profileName : new List<String>{ 'Standard User', 'Minimum Access - Salesforce', 'System Administrator' }) {
            Profile p = [SELECT Id FROM Profile WHERE Name = :profileName];
            String uniq = String.valueOf(Math.abs(Crypto.getRandomInteger()));
            User u = new User(Alias = 'v35p', Email = 'v35p' + uniq + '@example.com', EmailEncodingKey = 'UTF-8', LastName = 'V35', LanguageLocaleKey = 'en_US',
                LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', Username = 'v35p' + uniq + '@example.com');
            User viaJson = (User)JSON.deserialize(JSON.serialize(u), User.class);
            viaJson.Username = 'json' + viaJson.Username;
            insert u;
            insert viaJson;
            m.put(profileName + ' json', assignments(viaJson.Id));
            m.put(profileName, assignments(u.Id));
            Integer before = [SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId = :u.Id];
            m.put(profileName + ' count', before);
        }
        return m;
    }
}
