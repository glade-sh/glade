delete [SELECT Id FROM Prefs__c];
Account a = new Account(Name='Probe'); insert a;
System.assertEquals(false, [SELECT SetupFlag__c FROM Account WHERE Id=:a.Id].SetupFlag__c, 'no default boundary');
insert new Prefs__c(Name='Chosen', Flag__c=true);
System.assertEquals(true, [SELECT SetupFlag__c FROM Account WHERE Id=:a.Id].SetupFlag__c, 'explicit-name organization default');
