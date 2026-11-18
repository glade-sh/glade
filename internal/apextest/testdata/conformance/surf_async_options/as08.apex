Database.QueryLocator q = Database.getQueryLocator('SELECT Id FROM Account');
Database.Cursor c = Database.getCursor(q);
System.debug('OBS|loc=' + c.getNumRecords());
