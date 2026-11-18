insert new List<Account>{new Account(Name='bound'), new Account(Name='fld'), new Account(Name='loc')};
try { String n = 'bound'; throw new QueryException('x'); } catch (Exception e) { }
try { List<Account> r = Database.query('SELECT Name FROM Account WHERE Name = :n' + ' LIMIT 1'); System.debug('OUT:' + (r.isEmpty() ? 'none' : r[0].Name)); } catch (Exception e) { System.debug('OUT:' + (e.getTypeName() + ': ' + e.getMessage())); }
