try { List<Account> l = Database.query('SELECT Id FROM Account WHERE Name = :H423.s'); System.debug('R|ok'); } catch (Exception e) { System.debug('R|' + e.getTypeName() + ': ' + e.getMessage()); }
