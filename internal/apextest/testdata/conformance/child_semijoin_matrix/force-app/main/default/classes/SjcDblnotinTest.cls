@IsTest private class SjcDblnotinTest { @IsTest static void run() {
Account a = new Account(Name='Example'); insert a;
Contact c = new Contact(LastName='Contact', AccountId=a.Id); insert c;
Opportunity o1 = new Opportunity(Name='Selected', StageName='Prospecting', CloseDate=Date.today(), AccountId=a.Id);
Opportunity o2 = new Opportunity(Name='Other', StageName='Closed Won', CloseDate=Date.today(), AccountId=a.Id);
insert new List<Opportunity>{o1,o2};
insert new OpportunityContactRole(OpportunityId=o1.Id, ContactId=c.Id, Role='Decision Maker');
insert new Case(Subject='Example', AccountId=a.Id);
String stage = 'Prospecting';
List<Account> rows = [SELECT Id, (SELECT Id FROM Opportunities WHERE Id NOT IN (SELECT OpportunityId FROM OpportunityContactRole WHERE Role = 'Other') AND Id NOT IN (SELECT OpportunityId FROM OpportunityContactRole WHERE IsPrimary = true)), (SELECT Id FROM Contacts), (SELECT Id FROM Cases) FROM Account WHERE Id = :a.Id];
Integer got = rows[0].Opportunities.size();
System.assertEquals(2, got);
} }
