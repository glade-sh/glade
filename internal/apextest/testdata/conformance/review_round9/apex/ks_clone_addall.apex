Map<String,Schema.SObjectField> fm = Schema.SObjectType.KsA__c.fields.getMap();
Set<String> k = fm.keySet();
Set<String> c = k.clone(); c.addAll(new Set<String>{'aaa_x','zzz_x'});
System.debug('OBS|ks_clone_addall|' + c.size() + '|' + c);