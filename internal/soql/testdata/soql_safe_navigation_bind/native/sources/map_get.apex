Account seed = new Account(Name='NAV-MAP');
insert seed;
Map<String, String> names = new Map<String, String>{'k' => 'NAV-MAP'};
List<Account> rows = [SELECT Id FROM Account WHERE Name = :names?.get('k')];
System.assertEquals(1, rows.size());
System.debug(LoggingLevel.ERROR, 'OBS=rows:1');
delete seed;
