Account seed = new Account(Name='NAV-SPACE');
insert seed;
Account holder = seed;
List<Account> rows = [SELECT Id FROM Account WHERE Name = :holder ?. Name];
System.assertEquals(1, rows.size());
System.debug(LoggingLevel.ERROR, 'OBS=rows:' + rows.size());
delete seed;
