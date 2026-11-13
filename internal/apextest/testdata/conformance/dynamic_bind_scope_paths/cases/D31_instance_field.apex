try { H423 h = new H423(); List<Account> l = h.byField(); System.debug('R|ok'); } catch (Exception e) { System.debug('R|' + e.getTypeName() + ': ' + e.getMessage()); }
