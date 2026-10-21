Account seed = new Account(Name='NAV-QUESTION?MARK');
insert seed;
List<Account> rows = [SELECT Id FROM Account WHERE Name = 'NAV-QUESTION?MARK'];
System.assertEquals(1, rows.size());
System.debug(LoggingLevel.ERROR, 'OBS=rows:1');
delete seed;
