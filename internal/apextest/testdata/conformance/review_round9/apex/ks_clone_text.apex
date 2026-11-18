Map<String,Schema.SObjectField> fm = Schema.SObjectType.KsA__c.fields.getMap();
Set<String> k = fm.keySet();
Set<String> c = k.clone();
System.debug('OBS|ks_clone_text|' + c + '|' + new List<String>(c)[0]);