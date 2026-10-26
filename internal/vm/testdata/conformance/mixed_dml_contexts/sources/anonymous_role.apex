Savepoint sp=Database.setSavepoint();
System.debug('OBS|'+JSON.serialize(CaptureMixedDml.observe(true)));
Database.rollback(sp);
