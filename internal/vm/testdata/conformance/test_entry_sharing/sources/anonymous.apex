Savepoint sp=Database.setSavepoint();
insert new CapturePrivate__c(Name='Anonymous owned');
System.debug('OBS|'+JSON.serialize(new Map<String,Object>{'count'=>[SELECT Id FROM CapturePrivate__c].size()}));
Database.rollback(sp);
