List<String> names = new List<String>();
for (String key : Schema.getGlobalDescribe().keySet()) if (key.endsWith('__c')) names.add(key);
names.sort();
List<String> sets = new List<String>(SObjectType.Probe__c.FieldSets.getMap().keySet());
sets.sort();
System.debug('CAPTURE_OBJECTS=' + JSON.serialize(names));
System.debug('CAPTURE_FIELDSETS=' + JSON.serialize(sets));
