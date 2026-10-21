Account seed = new Account(Name='NAV-DYNAMIC');
insert seed;
OwnedSafeNavigationBindHolder holder = new OwnedSafeNavigationBindHolder();
holder.Name = 'NAV-DYNAMIC';
String query = 'SELECT Id FROM Account WHERE Name = :holder?.Name';
List<SObject> rows = Database.query(query);
System.debug(LoggingLevel.ERROR, 'OBS=rows:' + rows.size());
delete seed;
