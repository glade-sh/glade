delete [SELECT Id FROM Account WHERE Name='Delivery anonymous'];
insert new Account(Name='Delivery anonymous',Phone='111');
System.debug('OBS|'+JSON.serialize(new Map<String,Object>{'seeded'=>true}));
