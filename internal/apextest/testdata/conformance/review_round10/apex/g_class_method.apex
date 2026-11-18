KsA__c a = new KsA__c(Name='t'); insert a; Map<Id,KsA__c> m = new Map<Id,KsA__c>(new List<KsA__c>{a});
KeyAdd.run(m, a.Id);
System.debug('OBS|g_class_method|ok');