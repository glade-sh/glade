Set<String> k = Schema.SObjectType.KsA__c.fields.getMap().keySet();
k.removeAll(new Set<String>{'id'});
System.debug('OBS|ks_removeall|ok');