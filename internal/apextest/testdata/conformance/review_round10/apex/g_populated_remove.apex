KsA__c a = new KsA__c(Name='t'); insert a; a = [SELECT Id, Name FROM KsA__c WHERE Id = :a.Id];
Map<String,Object> pm = a.getPopulatedFieldsAsMap(); Integer n = pm.size(); pm.keySet().remove('Name');
System.debug('OBS|g_populated_remove|' + n + '|' + pm.size() + '|' + pm.containsKey('Name'));