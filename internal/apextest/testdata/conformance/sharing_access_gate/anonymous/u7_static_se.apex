List<Account> l = [SELECT Id FROM Account WITH SECURITY_ENFORCED LIMIT 1];
System.debug('RESULT n=' + l.size());
