Map<String,Schema.SObjectField> fm = Schema.SObjectType.KsA__c.fields.getMap();
Set<String> k = fm.keySet();
Integer before = fm.size(); k.remove('id');
Set<String> fresh = fm.keySet();
System.debug('OBS|ks_remove_effect|' + before + '|' + k.size() + '|' + k.contains('id') + '|' + fm.size() + '|' + fm.containsKey('id') + '|' + fresh.size() + '|' + fresh.contains('id'));