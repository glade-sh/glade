List<String> rows = new List<String>();
try {
 Account a = new Account(Phone='555', Rating='Warm', Name='Acme', AnnualRevenue=12.5, NumberOfEmployees=3, Website=null);
 rows.add('order=' + JSON.serialize(a));
} catch (Exception e) { rows.add('order=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('empty=' + JSON.serialize(new Account())); } catch (Exception e) { rows.add('empty=' + e.getTypeName() + '|' + e.getMessage()); }
try {
 Contact c = new Contact(LastName='Kid', Department='Ops');
 c.Account = new Account(Name='ParentCo', Website='p.example');
 rows.add('relNew=' + JSON.serialize(c));
} catch (Exception e) { rows.add('relNew=' + e.getTypeName() + '|' + e.getMessage()); }
try {
 Account p = new Account(Name='ParentCo');
 insert p;
 Contact c = new Contact(LastName='Kid', AccountId=p.Id);
 insert c;
 Account q = [SELECT Id, Name, (SELECT Id, LastName FROM Contacts) FROM Account WHERE Id=:p.Id];
 rows.add('queried=' + JSON.serialize(q));
} catch (Exception e) { rows.add('queried=' + e.getTypeName() + '|' + e.getMessage()); }
try {
 Account a = new Account(Name='RT', Website=null, AnnualRevenue=2.5);
 String s1 = JSON.serialize(a);
 Account back = (Account) JSON.deserialize(s1, Account.class);
 rows.add('rtSame=' + (s1 == JSON.serialize(back)));
 rows.add('rtIn=' + s1);
} catch (Exception e) { rows.add('rt=' + e.getTypeName() + '|' + e.getMessage()); }
for (String r : rows) { System.debug('OBS|' + r); }
