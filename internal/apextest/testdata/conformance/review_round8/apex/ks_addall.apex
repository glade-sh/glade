Set<String> k = Schema.SObjectType.KsA__c.fields.getMap().keySet();
k.addAll(new Set<String>{'zz'});
System.debug('OBS|ks_addall|ok');