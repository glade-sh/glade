Database.Cursor c = Database.getCursor('SELECT Id FROM Account');
c.setFetchSize(2);
System.debug('OBS|size=' + c.getFetchSize());
