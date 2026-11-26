Savepoint boundary = Database.setSavepoint();
System.debug('OBS|' + JSON.serialize(JuniperUpsertProbe.observe()));
Database.rollback(boundary);
