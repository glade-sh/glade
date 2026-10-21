Savepoint sp=Database.setSavepoint();System.debug('OBS|'+JSON.serialize(SecuredProjectionProbe.observe()));Database.rollback(sp);
