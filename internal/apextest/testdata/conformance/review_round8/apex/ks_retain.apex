Set<String> k = Schema.SObjectType.KsA__c.fields.getMap().keySet();
k.retainAll(new Set<String>{'id'});
System.debug('OBS|ks_retain|ok');