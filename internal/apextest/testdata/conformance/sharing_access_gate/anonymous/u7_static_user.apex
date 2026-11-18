List<Account> l = [SELECT Id FROM Account WITH USER_MODE LIMIT 1];
System.debug('RESULT n=' + l.size());
