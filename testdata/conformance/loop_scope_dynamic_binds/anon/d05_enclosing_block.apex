insert new List<Account>{new Account(Name='bound'), new Account(Name='fld'), new Account(Name='loc')};
String n = 'bound';
if (true) { List<Account> r = Database.query('SELECT Name FROM Account WHERE Name = :n' + ' LIMIT 1');
System.debug('OUT:' + (r.isEmpty() ? 'none' : r[0].Name)); }
