Set<String> k = Schema.SObjectType.KsA__c.fields.getMap().keySet();
System.debug('OBS|ks_a|' + k + '|' + String.valueOf(k).length());