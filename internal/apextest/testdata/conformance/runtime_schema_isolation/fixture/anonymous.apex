Savepoint boundary = Database.setSavepoint();
delete [SELECT Id FROM Account WHERE Name = 'Owned transient'];
System.debug('SCHEMA_ROW:' + SchemaIsolationProbe.observe());
Database.rollback(boundary);
