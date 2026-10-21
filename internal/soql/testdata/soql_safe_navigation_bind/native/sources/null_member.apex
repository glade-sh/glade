OwnedSafeNavigationBindHolder missing = null;
List<Account> rows = [SELECT Id FROM Account WHERE Name = :missing?.Name];
System.assertEquals(0, rows.size());
System.debug(LoggingLevel.ERROR, 'OBS=rows:0');
