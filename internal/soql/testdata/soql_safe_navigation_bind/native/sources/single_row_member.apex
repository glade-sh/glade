Account seed = new Account(Name='NAV-ONE');
insert seed;
Account holder = seed;
Id selected = [SELECT Id FROM Account WHERE Name = :holder?.Name]?.Id;
System.assertEquals(seed.Id, selected);
System.debug(LoggingLevel.ERROR, 'OBS=value:id');
delete seed;
