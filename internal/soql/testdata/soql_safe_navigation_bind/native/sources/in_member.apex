Account seed = new Account(Name='NAV-IN');
insert seed;
OwnedSafeNavigationBindHolder a = new OwnedSafeNavigationBindHolder();
a.ids = new List<Id>{seed.Id};
List<Account> rows = [SELECT Id FROM Account WHERE Id IN :a?.ids];
System.assertEquals(1, rows.size());
System.debug(LoggingLevel.ERROR, 'OBS=rows:1');
delete seed;
