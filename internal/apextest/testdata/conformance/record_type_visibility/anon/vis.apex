
Map<String,Object> m = new Map<String,Object>();
Savepoint sp = Database.setSavepoint();
Map<String,Schema.SObjectType> g = Schema.getGlobalDescribe();
Map<String,Map<String,Object>> req = new Map<String,Map<String,Object>>{
  'Account' => new Map<String,Object>{'Name' => 'RtVis'},
  'Contact' => new Map<String,Object>{'LastName' => 'RtVis'},
  'Opportunity' => new Map<String,Object>{'Name' => 'RtVis', 'StageName' => 'Prospecting', 'CloseDate' => Date.newInstance(2030, 1, 1)},
  'V35NoRt__c' => new Map<String,Object>{'Name' => 'RtVis'},
  'V35Rt__c' => new Map<String,Object>{'Name' => 'RtVis'}};
Map<Id,String> rtNames = new Map<Id,String>();
Map<String,Id> rtIds = new Map<String,Id>();
Map<String,List<String>> rtByObj = new Map<String,List<String>>();
for (String o : req.keySet()) { rtByObj.put(o, new List<String>()); }
for (RecordType rt : [SELECT Id, DeveloperName, SobjectType FROM RecordType WHERE SobjectType IN :req.keySet() ORDER BY DeveloperName]) { rtNames.put(rt.Id, rt.DeveloperName); rtIds.put(rt.SobjectType + '.' + rt.DeveloperName, rt.Id); rtByObj.get(rt.SobjectType).add(rt.DeveloperName); }
for (String obj : new List<String>{'Account','Contact','Opportunity','V35NoRt__c','V35Rt__c'}) {
  Schema.SObjectType t = g.get(obj);
  Schema.DescribeSObjectResult d = t.getDescribe();
  List<String> infos = new List<String>();
  for (Schema.RecordTypeInfo i : d.getRecordTypeInfos()) { infos.add(i.getDeveloperName() + ':' + i.isAvailable() + ':' + i.isDefaultRecordTypeMapping() + ':' + i.isMaster()); }
  infos.sort();
  m.put(obj + '.infos', infos);
  m.put(obj + '.hasField', d.fields.getMap().containsKey('recordtypeid'));
  List<String> rtRows = new List<String>();
  List<String> variants = new List<String>{'omit', 'null', 'master'};
  for (String dn : rtByObj.get(obj)) { rtRows.add(dn); variants.add('rt:' + dn); }
  m.put(obj + '.rtRows', rtRows);
  Map<String,String> soql = new Map<String,String>{
    'selectId' => 'SELECT RecordTypeId FROM ' + obj + ' LIMIT 1',
    'selectName' => 'SELECT RecordType.Name FROM ' + obj + ' LIMIT 1',
    'whereId' => 'SELECT Id FROM ' + obj + ' WHERE RecordTypeId = null LIMIT 1',
    'whereName' => 'SELECT Id FROM ' + obj + ' WHERE RecordType.Name = \'x\' LIMIT 1',
    'orderId' => 'SELECT Id FROM ' + obj + ' ORDER BY RecordTypeId LIMIT 1',
    'orderName' => 'SELECT Id FROM ' + obj + ' ORDER BY RecordType.Name LIMIT 1',
    'groupId' => 'SELECT RecordTypeId, COUNT(Id) n FROM ' + obj + ' GROUP BY RecordTypeId',
    'count' => 'SELECT COUNT() FROM ' + obj + ' WHERE RecordTypeId != null'};
  for (String k : soql.keySet()) {
    try {
      if (k == 'count') { Database.countQuery(soql.get(k)); } else { Database.query(soql.get(k)); }
      m.put(obj + '.soql.' + k, 'ok');
    } catch (Exception e) { m.put(obj + '.soql.' + k, e.getTypeName() + ':' + e.getMessage()); }
  }
  for (String v : variants) {
    String key = obj + '.dml.' + v;
    SObject r = t.newSObject();
    for (String f : req.get(obj).keySet()) { r.put(f, req.get(obj).get(f)); }
    try {
      if (v == 'null') { r.put('RecordTypeId', null); }
      if (v == 'master') { r.put('RecordTypeId', '012000000000000AAA'); }
      if (v.startsWith('rt:')) { r.put('RecordTypeId', rtIds.get(obj + '.' + v.substring(3))); }
    } catch (Exception e) { m.put(key, 'put ' + e.getTypeName() + ':' + e.getMessage()); continue; }
    try {
      insert r;
      String got = 'n/a';
      try { SObject back = Database.query('SELECT RecordTypeId FROM ' + obj + ' WHERE Id = \'' + r.Id + '\''); Id rid = (Id) back.get('RecordTypeId'); got = rid == null ? 'null' : (rtNames.containsKey(rid) ? rtNames.get(rid) : String.valueOf(rid)); } catch (Exception e) { got = 'n/a'; }
      m.put(key, 'ok ' + got);
      if (v == 'omit') {
        List<String> ups = new List<String>{'null'};
        for (String rv : rtRows) { ups.add(rv); }
        for (String u : ups) {
          SObject w = t.newSObject(r.Id);
          try {
            Id target = null;
            if (u != 'null') { target = rtIds.get(obj + '.' + u); }
            w.put('RecordTypeId', target);
            update w;
            SObject back = Database.query('SELECT RecordTypeId FROM ' + obj + ' WHERE Id = \'' + r.Id + '\'');
            Id rid = (Id) back.get('RecordTypeId');
            m.put(obj + '.update.' + u, 'ok ' + (rid == null ? 'null' : rtNames.get(rid)));
          } catch (Exception e) { m.put(obj + '.update.' + u, e.getTypeName() + ':' + e.getMessage().replaceAll('012[A-Za-z0-9]{12,15}', '<rtid>')); }
        }
      }
    } catch (Exception e) { m.put(key, e.getTypeName() + ':' + e.getMessage().replaceAll('012[A-Za-z0-9]{12,15}', '<rtid>')); }
    if (v == 'null') {
      SObject p = t.newSObject();
      for (String f : req.get(obj).keySet()) { p.put(f, req.get(obj).get(f)); }
      p.put('RecordTypeId', null);
      Database.SaveResult sr = Database.insert(p, false);
      m.put(obj + '.partialNull', sr.isSuccess() ? 'ok' : String.valueOf(sr.getErrors()[0].getStatusCode()) + ' ' + sr.getErrors()[0].getMessage() + ' ' + sr.getErrors()[0].getFields());
    }
  }
}
Database.rollback(sp);
System.debug('OBS|' + JSON.serialize(m));
