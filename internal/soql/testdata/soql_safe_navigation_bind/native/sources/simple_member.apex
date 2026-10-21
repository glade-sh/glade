Account seed = new Account(Name='NAV-SIMPLE');
insert seed;
Account holder = seed;
List<Account> rows = [SELECT Id FROM Account WHERE Name = :holder?.Name];
System.assertEquals(1, rows.size());
System.debug(LoggingLevel.ERROR, 'OBS=rows:1');
delete seed;
