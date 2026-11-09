public class C4PortalProbe {
    public static Integer seq = 0;
    public static String uniqueName(String prefix) {
        seq++;
        return prefix + seq + '.' + Math.abs(Crypto.getRandomInteger()) + '@example.com';
    }
    public static User newUser(String alias, Id profileId, Id roleId, Id contactId) {
        return new User(Alias = alias, Email = alias + '@example.com', EmailEncodingKey = 'UTF-8', LastName = alias,
            LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', TimeZoneSidKey = 'America/Los_Angeles',
            ProfileId = profileId, UserRoleId = roleId, ContactId = contactId, UserName = uniqueName(alias));
    }
    public static Id profileId(String name) {
        return [SELECT Id FROM Profile WHERE Name = :name LIMIT 1].Id;
    }
    // Shares and record access, read in system context.
    public static Map<String, Object> shares(List<Account> accounts, User portal, User owner) {
        Map<Id, String> names = new Map<Id, String>();
        for (Account a : accounts) { names.put(a.Id, a.Name); }
        List<AccountShare> rows = [SELECT AccountId, RowCause, AccountAccessLevel, UserOrGroupId FROM AccountShare WHERE AccountId IN :names.keySet()];
        Set<Id> groupIds = new Set<Id>();
        for (AccountShare s : rows) { groupIds.add(s.UserOrGroupId); }
        Map<Id, Group> groups = new Map<Id, Group>([SELECT Id, Type, RelatedId FROM Group WHERE Id IN :groupIds]);
        Set<Id> roleIds = new Set<Id>();
        for (Group g : groups.values()) { if (g.RelatedId != null) roleIds.add(g.RelatedId); }
        Map<Id, UserRole> roles = new Map<Id, UserRole>([SELECT Id, PortalType, PortalRole FROM UserRole WHERE Id IN :roleIds]);
        List<String> out = new List<String>();
        for (AccountShare s : rows) {
            String who;
            if (s.UserOrGroupId == portal.Id) who = 'portalUser';
            else if (owner != null && s.UserOrGroupId == owner.Id) who = 'owner';
            else if (groups.containsKey(s.UserOrGroupId)) {
                Group g = groups.get(s.UserOrGroupId);
                UserRole r = roles.get(g.RelatedId);
                who = 'group:' + g.Type + (r == null ? '' : (r.PortalType == 'None' ? ':internalRole' : ':' + r.PortalType + '/' + r.PortalRole));
            } else who = 'other';
            out.add(names.get(s.AccountId) + '|' + s.RowCause + '|' + s.AccountAccessLevel + '|' + who);
        }
        out.sort();
        Map<String, Object> access = new Map<String, Object>();
        for (UserRecordAccess a : [SELECT RecordId, HasReadAccess FROM UserRecordAccess WHERE UserId = :portal.Id AND RecordId IN :names.keySet()]) {
            access.put(names.get(a.RecordId), a.HasReadAccess);
        }
        User stored = [SELECT UserType, UserRoleId, UserRole.PortalType, UserRole.PortalRole, IsPortalEnabled FROM User WHERE Id = :portal.Id];
        return new Map<String, Object>{'shares' => out, 'readAccess' => access, 'userType' => stored.UserType,
            'portalRole' => stored.UserRoleId == null ? 'none' : stored.UserRole.PortalType + '/' + stored.UserRole.PortalRole,
            'isPortalEnabled' => stored.IsPortalEnabled};
    }
    public static List<String> visibleAccounts() {
        List<String> out = new List<String>();
        for (Account a : [SELECT Name FROM Account ORDER BY Name]) out.add(a.Name);
        return out;
    }
    public static List<String> visibleContacts() {
        List<String> out = new List<String>();
        for (Contact c : [SELECT LastName FROM Contact ORDER BY LastName]) out.add(c.LastName);
        return out;
    }
}
