List<Account> rows = [SELECT Id, (SELECT Id FROM Contacts WHERE Id IN (SELECT ContactId FROM OpportunityContactRole) OR LastName = 'Example') FROM Account];
