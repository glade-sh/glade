Set<String> k = Schema.SObjectType.KsA__c.fields.getMap().keySet();
Set<String> c = new Set<String>(k);
System.debug('OBS|ks_copy|' + c + '|' + new List<String>(k)[0] + ',' + new List<String>(k)[1]);