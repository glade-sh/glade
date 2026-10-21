Account seed = new Account(Name='NAV-CHAIN');
insert seed;
OwnedSafeNavigationBindHolder a = new OwnedSafeNavigationBindHolder();
a.b = new OwnedSafeNavigationBindHolder();
a.b.Name = 'NAV-CHAIN';
List<Account> rows = [SELECT Id FROM Account WHERE Name = :a?.b?.Name];
System.assertEquals(1, rows.size());
System.debug(LoggingLevel.ERROR, 'OBS=rows:1');
delete seed;
