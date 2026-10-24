Savepoint checkpoint=Database.setSavepoint();
System.debug('OBS|'+JSON.serialize(MeadowDirectProbe.observe()));
Database.rollback(checkpoint);
