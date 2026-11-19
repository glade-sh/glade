try { Integer n = Database.countQuery('SELECT COUNT() FROM Account WHERE Name = :nope'); System.debug('R|ok'); } catch (Exception e) { System.debug('R|' + e.getTypeName() + ': ' + e.getMessage()); }
