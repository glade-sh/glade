delete [SELECT Id FROM EqualityProbe__c];
System.debug('CASE349_EQUALITY|' + Case349Probe.equalityProbe());
System.debug('CASE349_ACCOUNT|' + Case349Probe.accountProbe());
