insert new List<PrivateProbe__c>{new PrivateProbe__c(Name='A'),new PrivateProbe__c(Name='B'),new PrivateProbe__c(Name='C')};
Database.QueryLocatorIterator it=Database.getQueryLocator('SELECT Id FROM PrivateProbe__c ORDER BY Name LIMIT 2').iterator(); Integer n=0; while(it.hasNext()){it.next();n++;} System.assert(false,'OBSERVED|anonymous|'+n);
