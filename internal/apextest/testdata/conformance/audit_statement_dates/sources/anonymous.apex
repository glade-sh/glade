insert new Account(Name='Cedar setup');
System.debug('PROBE|'+JSON.serialize(CedarAuditProbe.observe(false)));
delete [SELECT Id FROM Account WHERE Name LIKE 'Cedar %'];
