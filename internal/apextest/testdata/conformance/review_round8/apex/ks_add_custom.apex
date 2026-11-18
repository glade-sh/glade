Set<String> k = Schema.SObjectType.KsA__c.fields.getMap().keySet();
k.add('aaa_added'); k.add('zzz_added');
System.debug('OBS|ks_add_custom|' + k + '|' + k.size() + '|' + k.contains('aaa_added'));