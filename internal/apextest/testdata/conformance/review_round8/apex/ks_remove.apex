Set<String> k = Schema.SObjectType.KsA__c.fields.getMap().keySet();
k.remove('id');
System.debug('OBS|ks_remove|ok');