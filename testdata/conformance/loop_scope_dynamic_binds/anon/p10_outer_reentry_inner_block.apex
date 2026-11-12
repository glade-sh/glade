insert new List<Account>{new Account(Name='bound'), new Account(Name='fld'), new Account(Name='loc')};
for (Integer i = 0; i < 2; i++) {
 if (i == 1) { try { List<Account> r = Database.query('SELECT Name FROM Account WHERE Name = :n' + ' LIMIT 1'); System.debug('OUT:' + (r.isEmpty() ? 'none' : r[0].Name)); } catch (Exception e) { System.debug('OUT:' + (e.getTypeName() + ': ' + e.getMessage())); } }
 { String n = 'bound'; }
}
