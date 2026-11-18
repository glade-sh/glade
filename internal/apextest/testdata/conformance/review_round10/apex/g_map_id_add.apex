KsA__c a = new KsA__c(Name='t'); insert a; Map<Id,KsA__c> m = new Map<Id,KsA__c>(new List<KsA__c>{a});
m.keySet().add(a.Id);
System.debug('OBS|g_map_id_add|ok');