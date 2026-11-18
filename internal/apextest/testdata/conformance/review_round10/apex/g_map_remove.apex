KsA__c a = new KsA__c(Name='t'); insert a; Map<Id,KsA__c> m = new Map<Id,KsA__c>(new List<KsA__c>{a});
Set<String> dummy; Integer n = m.size(); m.keySet().remove(a.Id);
System.debug('OBS|g_map_remove|' + n + '|' + m.size());