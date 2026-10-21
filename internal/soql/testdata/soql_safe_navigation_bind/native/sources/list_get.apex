Account seed = new Account(Name='NAV-LIST');
insert seed;
List<String> names = new List<String>{'NAV-LIST'};
List<Account> rows = [SELECT Id FROM Account WHERE Name = :names?.get(0)];
System.assertEquals(1, rows.size());
System.debug(LoggingLevel.ERROR, 'OBS=rows:1');
delete seed;
