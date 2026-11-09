@IsTest private class Q3Synthetic62 {
 private class Rank implements Comparable { Integer n; Rank(Integer x){n=x;} public Integer compareTo(Object other){return n-((Rank)other).n;} }
 private class Box { public Event a; Box(){a=new Event(); a.StartDateTime=Datetime.newInstanceGMT(2020,1,2,3,4,5);} }
 @IsTest static void child() {
Account a = new Account(Name='Q3 synthetic'); insert a;
Contact c1 = new Contact(LastName='Q3 one', AccountId=a.Id); Contact c2 = new Contact(LastName='Q3 two', AccountId=a.Id); insert new List<Contact>{c1,c2};
Opportunity o = new Opportunity(Name='Q3 opportunity', StageName='Prospecting', CloseDate=Date.today(), AccountId=a.Id); insert o;
insert new OpportunityContactRole(OpportunityId=o.Id, ContactId=c1.Id, Role='Decision Maker');
List<Account> yes = [SELECT Id, (SELECT LastName FROM Contacts WHERE Id IN (SELECT ContactId FROM OpportunityContactRole WHERE Role = 'Decision Maker')) FROM Account WHERE Id = :a.Id];
List<Account> no = [SELECT Id, (SELECT LastName FROM Contacts WHERE Id NOT IN (SELECT ContactId FROM OpportunityContactRole WHERE Role = 'Decision Maker')) FROM Account WHERE Id = :a.Id];
System.assertEquals(1, yes.size()); System.assertEquals(1, yes[0].Contacts.size()); System.assertEquals('Q3 one',yes[0].Contacts[0].LastName);
System.assertEquals(1, no.size()); System.assertEquals(1, no[0].Contacts.size()); System.assertEquals('Q3 two',no[0].Contacts[0].LastName);
System.debug('Q3_RESULT=pass');

 }
 @IsTest static void bundle() {
  List<Rank> vals = new List<Rank>{new Rank(3),new Rank(1),new Rank(2)}; vals.sort(); System.assertEquals(1,vals[0].n); System.assertEquals(3,vals[2].n);
  Box b=new Box(); System.assertEquals(1577934245000L,b.a.StartDateTime.getTime());
  System.assertEquals('/apex/q3mixedcase',Page.Q3MixedCase.getUrl());
  Schema.SObjectField tok=Account.Name; System.assertEquals('Name',tok.getDescribe().getName());
 }
}