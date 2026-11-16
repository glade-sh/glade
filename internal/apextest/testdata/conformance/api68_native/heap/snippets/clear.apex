delete [SELECT Id FROM Capture_Row__c WHERE Case__c = 'heap'];
System.debug(LoggingLevel.ERROR, 'CLEARED=true');
