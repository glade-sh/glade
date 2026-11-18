KsA__c a = new KsA__c(Name='t'); insert a; a = [SELECT Id, Name FROM KsA__c WHERE Id = :a.Id];
Map<String,Object> pm = a.getPopulatedFieldsAsMap(); pm.keySet().add('zz');
System.debug('OBS|g_populated_add|ok');