Map<String,Schema.SObjectField> fm = Schema.SObjectType.KsA__c.fields.getMap();
Set<String> k = fm.keySet();
Map<String,Schema.SObjectField> cp = fm.clone();
Set<String> ck = cp.keySet();
System.debug('OBS|ks_map_clone|' + ck + '|' + new List<String>(ck)[0] + '|' + cp.size());