Set<String> k = Schema.SObjectType.KsA__c.fields.getMap().keySet();
List<String> l = new List<String>(k); String s='';
for (Integer i=0;i<Math.min(8,l.size());i++) s += l[i] + ',';
System.debug('OBS|ks_iter|' + s);