Map<String,Schema.SObjectField> fm = Schema.SObjectType.KsA__c.fields.getMap();
Set<String> k = fm.keySet();
Set<String> u = new Set<String>(); u.addAll(k);
System.debug('OBS|ks_u_addall|' + u + '|' + new List<String>(u)[0]);