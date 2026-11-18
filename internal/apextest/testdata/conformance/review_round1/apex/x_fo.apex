List<String> a = new List<String>(Fo__c.SObjectType.getDescribe().fields.getMap().keySet());
System.debug('OBS|fo|' + String.join(a, ','));