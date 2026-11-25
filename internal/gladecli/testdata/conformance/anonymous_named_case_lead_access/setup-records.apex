Id owner = UserInfo.getUserId();
Account a = new Account(Name='Sharing Probe Account', OwnerId=owner); insert a;
Contact c = new Contact(LastName='Sharing Probe Contact', AccountId=a.Id, OwnerId=owner); insert c;
insert new List<Case>{new Case(Subject='Sharing Probe Other', OwnerId=owner, Status='New', Origin='Web'), new Case(Subject='Sharing Probe Contact Case', OwnerId=owner, ContactId=c.Id, AccountId=a.Id, Status='New', Origin='Web')};
insert new Lead(LastName='Sharing Probe Lead', Company='Sharing Probe', OwnerId=owner);
