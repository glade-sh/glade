Map<String,Object> o = new Map<String,Object>();
try { update new User(Id = UserInfo.getUserId(), Title = 'Admin Title'); o.put('byIdTitle', 'ok'); } catch (Exception e) { o.put('byIdTitle', e.getTypeName() + ': ' + e.getMessage()); }
try {
    User q = [SELECT Id, ProfileId, Alias, LastName, Username, Email, TimeZoneSidKey, LocaleSidKey, EmailEncodingKey, LanguageLocaleKey FROM User WHERE Id = :UserInfo.getUserId()];
    update q; o.put('queriedInsertFields', 'ok');
} catch (Exception e) { o.put('queriedInsertFields', e.getTypeName() + ': ' + e.getMessage()); }
System.debug(LoggingLevel.ERROR, 'CAP|anon.admin|' + JSON.serialize(o));
