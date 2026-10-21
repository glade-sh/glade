Savepoint sp=Database.setSavepoint();System.debug('OBS|'+JSON.serialize(QueryProjectionProbe.observe()));Database.rollback(sp);
