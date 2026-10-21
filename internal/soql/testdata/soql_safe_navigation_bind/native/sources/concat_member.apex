Account seed = new Account(Name='NAV-CONCATx');
insert seed;
Account holder = new Account(Name='NAV-CONCAT');
List<Account> rows = [SELECT Id FROM Account WHERE Name = :holder?.Name + 'x'];
System.assertEquals(1, rows.size());
System.debug(LoggingLevel.ERROR, 'OBS=rows:1');
delete seed;
