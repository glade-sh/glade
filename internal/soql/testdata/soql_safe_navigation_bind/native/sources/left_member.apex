Account seed = new Account(Name='NAV-LEFT');
insert seed;
Account holder = seed;
List<Account> rows = [SELECT Id FROM Account WHERE :holder?.Name = Name];
System.debug(LoggingLevel.ERROR, 'OBS=rows:' + rows.size());
delete seed;
