try { String callerLocal='x'; List<Account> l = H423.run(); System.debug('R|ok'); } catch (Exception e) { System.debug('R|' + e.getTypeName() + ': ' + e.getMessage()); }
