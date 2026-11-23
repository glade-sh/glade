@IsTest private class DynamicBindProbe { @IsTest static void observe() { Account a=new Account(Name='Probe Parent'); insert a;
Contact hit=new Contact(LastName='Hit',FirstName='First',AccountId=a.Id); Contact miss=new Contact(LastName='Miss',AccountId=a.Id); insert new List<Contact>{hit,miss};
insert new Case(Subject='Probe',Status='New',ContactId=hit.Id);
String lastName='Hit', firstName='First', subject='Probe', parentName='Probe Parent';
String result; try { List<SObject> rows=Database.query('SELECT Id, (SELECT Id FROM Opportunities LIMIT 1), (SELECT Id, LastName, FirstName, Account.Name FROM Contacts WHERE LastName = :lastName AND FirstName = :firstName AND Id IN (SELECT ContactId FROM Case WHERE Status = \'New\' AND Subject = :subject) LIMIT 1), (SELECT Id FROM Cases LIMIT 1), (SELECT Id FROM Assets LIMIT 1) FROM Account WHERE Name = :parentName'); List<SObject> contacts=rows[0].getSObjects('Contacts'); result='rows='+rows.size()+';contacts='+(contacts==null?0:contacts.size())+';name='+(contacts==null?'none':String.valueOf(contacts[0].get('LastName'))); } catch (Exception e) { result=e.getTypeName()+':'+e.getMessage(); }
System.assert(false,'OBS:'+result); } }
