delete [SELECT Id FROM Addr_Test__c];
delete [SELECT Id FROM Account WHERE Name = 'acct'];
for (String r : AddressFieldProbe.run()) System.debug(LoggingLevel.ERROR, 'ADDR|' + r);
