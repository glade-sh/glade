Set<String> k = Schema.SObjectType.KsA__c.fields.getMap().keySet();
Set<String> c = new Set<String>(k); c.add('aaa_added'); c.add('zzz_added');
System.debug('OBS|ks_copy_add|' + c.size() + '|' + c);