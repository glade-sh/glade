insert new AsyncParcel__c(Name='async',Source__c='initial');
AsyncParcel__c row=[SELECT Id,Source__c FROM AsyncParcel__c LIMIT 1];row.Source__c='changed';update row;
System.debug('ASYNC_BEFORE|'+[SELECT count() FROM AsyncParcel__c WHERE Delayed__c=true]);
