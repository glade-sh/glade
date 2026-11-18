Map<String,Schema.SObjectField> fm = Schema.SObjectType.KsA__c.fields.getMap();
Set<String> k = fm.keySet();
Set<String> u = new Set<String>(new List<String>(k));
System.debug('OBS|ks_list_set|' + u + '|' + new List<String>(u)[0]);