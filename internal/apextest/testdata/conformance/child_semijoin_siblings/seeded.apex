Account a = new Account(Name='Example'); insert a;
Contact c = new Contact(LastName='Contact', AccountId=a.Id, Email='contact@example.invalid'); insert c;
Opportunity o = new Opportunity(Name='Selected', StageName='Prospecting', CloseDate=Date.today(), AccountId=a.Id); insert o;
insert new OpportunityContactRole(OpportunityId=o.Id, ContactId=c.Id, Role='Decision Maker');
insert new Case(Subject='Example', AccountId=a.Id);
List<Account> rows = [SELECT Id, (SELECT Id FROM Contacts WHERE Email != null), (SELECT Id, Name FROM Opportunities WHERE StageName = 'Prospecting' AND Id IN (SELECT OpportunityId FROM OpportunityContactRole WHERE Role = 'Decision Maker') LIMIT 1), (SELECT Id FROM Cases LIMIT 1) FROM Account WHERE Id = :a.Id];
System.assertEquals(1, rows.size()); System.assertEquals(1, rows[0].Contacts.size()); System.assertEquals(1, rows[0].Opportunities.size()); System.assertEquals('Selected', rows[0].Opportunities[0].Name); System.assertEquals(1, rows[0].Cases.size());
System.debug('SEEDED=pass');
