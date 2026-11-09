delete [SELECT Id FROM Prefs__c];
insert new Prefs__c(Name='Organization', SetupOwnerId=UserInfo.getOrganizationId(), Flag__c=true);
insert new Prefs__c(Name='Profile', SetupOwnerId=UserInfo.getProfileId(), Flag__c=false);
Account a = new Account(Name='Probe'); insert a;
System.assertEquals(false, [SELECT SetupFlag__c FROM Account WHERE Id=:a.Id].SetupFlag__c, 'profile overrides org');
