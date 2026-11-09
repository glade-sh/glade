Map<String,Object> m = new Map<String,Object>();
Opportunity o = new Opportunity(Name='x');
try { o.put('RecordTypeId', null); m.put('put', 'ok'); } catch (Exception e) { m.put('put', e.getTypeName() + ':' + e.getMessage()); }
try { m.put('get', String.valueOf(o.get('RecordTypeId'))); } catch (Exception e) { m.put('get', e.getTypeName() + ':' + e.getMessage()); }
m.put('hasField', Opportunity.SObjectType.getDescribe().fields.getMap().containsKey('recordtypeid'));
List<String> infos = new List<String>();
for (Schema.RecordTypeInfo i : Opportunity.SObjectType.getDescribe().getRecordTypeInfos()) infos.add(i.getDeveloperName() + ':' + i.isAvailable() + ':' + i.isDefaultRecordTypeMapping() + ':' + i.isMaster());
infos.sort(); m.put('infos', infos);
Account a = new Account(Name='y');
try { a.put('RecordTypeId', null); m.put('accountPut', 'ok'); } catch (Exception e) { m.put('accountPut', e.getTypeName() + ':' + e.getMessage()); }
System.debug('OBS|' + JSON.serialize(m));
