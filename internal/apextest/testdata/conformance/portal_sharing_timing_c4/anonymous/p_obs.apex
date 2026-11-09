// Observes AccountShare and UserRecordAccess for the __P__ portal users (system context).
Map<String, Object> o = new Map<String, Object>();
User admin = [SELECT Id FROM User WHERE Id = :UserInfo.getUserId()];
for (String lic : new List<String>{'cc', 'ccp'}) {
    List<Account> accts = [SELECT Id, Name FROM Account WHERE Name LIKE :('__P__' + lic + '%') ORDER BY Name];
    List<User> us = [SELECT Id FROM User WHERE LastName = :('__P__' + lic + 'prt')];
    if (us.isEmpty()) { o.put(lic, 'nouser'); continue; }
    o.put(lic, C4PortalProbe.shares(accts, us[0], admin));
}
System.debug('CAP|p_obs|' + JSON.serialize(o).replace('__P__', ''));
