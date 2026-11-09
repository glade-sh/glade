// Sets the running admin's LocaleSidKey in its own transaction. The caller
// substitutes __LOCALE__ before running.
User me = new User(Id = UserInfo.getUserId(), LocaleSidKey = '__LOCALE__');
update me;
System.debug('CAP|set_locale|' + JSON.serialize(new Map<String,Object>{
    'set' => '__LOCALE__',
    'getLocale' => UserInfo.getLocale(),
    'queried' => [SELECT LocaleSidKey FROM User WHERE Id = :UserInfo.getUserId()].LocaleSidKey}));
