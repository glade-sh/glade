// Inserts the portal users, then observes in the same transaction.
Map<String, String> profiles = new Map<String, String>{'cc' => 'Customer Community User', 'ccp' => 'Customer Community Plus User'};
for (String lic : profiles.keySet()) {
    Contact c = [SELECT Id FROM Contact WHERE LastName = :('__P__' + lic + 'Contact')];
    User u = C4PortalProbe.newUser('c4' + lic.left(3), C4PortalProbe.profileId(profiles.get(lic)), null, c.Id);
    u.LastName = '__P__' + lic + 'prt';
    insert u;
}
Map<String, Object> o = new Map<String, Object>();
User admin = [SELECT Id FROM User WHERE Id = :UserInfo.getUserId()];
for (String lic : profiles.keySet()) {
    List<Account> accts = [SELECT Id, Name FROM Account WHERE Name LIKE :('__P__' + lic + '%') ORDER BY Name];
    User u = [SELECT Id FROM User WHERE LastName = :('__P__' + lic + 'prt')];
    o.put(lic, C4PortalProbe.shares(accts, u, admin));
}
System.debug('CAP|p_user|' + JSON.serialize(o).replace('__P__', ''));
