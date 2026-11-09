Map<String,Object> m = new Map<String,Object>();
Savepoint sp = Database.setSavepoint();
Id beta = [SELECT Id FROM RecordType WHERE SobjectType = 'V35Rt__c' AND DeveloperName = 'Beta'].Id;
for (String v : new List<String>{'betaPlain', 'betaAsSystem', 'betaAsUser', 'betaDbSystem', 'betaDbUser', 'nullPlain', 'nullAsSystem', 'nullAsUser', 'nullDbSystem', 'nullDbUser'}) {
  V35Rt__c r = new V35Rt__c(Name = 'Mode');
  r.put('RecordTypeId', v.startsWith('beta') ? beta : null);
  try {
    if (v.endsWith('Plain')) { insert r; }
    if (v.endsWith('AsSystem')) { insert as system r; }
    if (v.endsWith('AsUser')) { insert as user r; }
    if (v.endsWith('DbSystem')) { Database.insert(r, AccessLevel.SYSTEM_MODE); }
    if (v.endsWith('DbUser')) { Database.insert(r, AccessLevel.USER_MODE); }
    m.put(v, 'ok');
  } catch (Exception e) { m.put(v, e.getTypeName() + ':' + e.getMessage().replaceAll('012[A-Za-z0-9]{12,15}', '<rtid>')); }
}
Database.rollback(sp);
System.debug('OBS|' + JSON.serialize(m));
