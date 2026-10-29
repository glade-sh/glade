delete [SELECT Id FROM Addr_Test__c];
delete [SELECT Id FROM Account WHERE Name IN ('ja1','ja2')];
for (String r : AddressFollowup.run()) System.debug(LoggingLevel.ERROR, 'ADDR|' + r);
