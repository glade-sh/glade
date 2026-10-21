Savepoint point = Database.setSavepoint();
System.debug('OBS|' + JSON.serialize(BulkReferenceProbe.observe()));
Database.rollback(point);
