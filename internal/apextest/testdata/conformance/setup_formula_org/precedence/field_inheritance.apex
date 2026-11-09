delete [SELECT Id FROM Prefs__c];
insert new Prefs__c(Name='Organization', SetupOwnerId=UserInfo.getOrganizationId(), Tone__c='org');
insert new Prefs__c(Name='Profile', SetupOwnerId=UserInfo.getProfileId(), Tone__c='profile');
insert new Prefs__c(Name='User', SetupOwnerId=UserInfo.getUserId(), Flag__c=true);
Account a = new Account(Name='Probe'); insert a;
System.assert(false, 'OBS|' + String.valueOf([SELECT SetupTone__c FROM Account WHERE Id=:a.Id].SetupTone__c));
