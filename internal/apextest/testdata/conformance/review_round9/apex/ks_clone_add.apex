Map<String,Schema.SObjectField> fm = Schema.SObjectType.KsA__c.fields.getMap();
Set<String> k = fm.keySet();
Set<String> c = k.clone(); c.add('aaa_x');
System.debug('OBS|ks_clone_add|' + c.size() + '|' + c);