Account acct = new Account(Name = 'amtest qzx', Phone = '5550101');
insert acct;
Profile prof = [SELECT Id FROM Profile WHERE Name = 'Minimum Access - Salesforce' LIMIT 1];
User usr = new User(Alias = 'amt', Email = 'amt@example.com', EmailEncodingKey = 'UTF-8', LastName = 'Amt', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', ProfileId = prof.Id, TimeZoneSidKey = 'America/Los_Angeles', UserName = 'amt' + String.valueOf(Crypto.getRandomInteger()) + '@example.com');
User usr2 = new User(Alias = 'amt2', Email = 'amt2@example.com', EmailEncodingKey = 'UTF-8', LastName = 'Amt2', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', ProfileId = prof.Id, TimeZoneSidKey = 'America/Los_Angeles', UserName = 'amt2' + String.valueOf(Crypto.getRandomInteger()) + '@example.com');
System.runAs(new User(Id = UserInfo.getUserId())) {
    insert new List<User>{ usr, usr2 };
    insert new PermissionSetAssignment(AssigneeId = usr2.Id, PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = 'EdgeAccountRead' LIMIT 1].Id);
}
Test.setFixedSearchResults(new List<Id>{ acct.Id });
