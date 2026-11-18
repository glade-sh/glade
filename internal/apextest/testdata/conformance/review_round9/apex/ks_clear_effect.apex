Map<String,Schema.SObjectField> fm = Schema.SObjectType.KsA__c.fields.getMap();
Set<String> k = fm.keySet();
k.clear();
Set<String> fresh = fm.keySet();
System.debug('OBS|ks_clear_effect|' + k.size() + '|' + fm.size() + '|' + fresh.size());