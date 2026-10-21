Account seed = new Account(Name='NAV-LIMIT');
insert seed;
OwnedSafeNavigationBindHolder a = new OwnedSafeNavigationBindHolder();
a.n = 1;
List<Account> rows = [SELECT Id FROM Account LIMIT :a?.n];
System.assertEquals(1, rows.size());
System.debug(LoggingLevel.ERROR, 'OBS=rows:1');
delete seed;
