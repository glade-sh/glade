Savepoint sp=Database.setSavepoint(); System.debug('TEXT_DEFAULT_RESULT|'+JSON.serialize(TextDefaultProbe.observe())); Database.rollback(sp);
