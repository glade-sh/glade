Account a = new Account(Name = 'S68-Q4', Age__c = 11, gladetest1__Age__c = 22);
insert a;
System.debug('Q4_68=' + gladetest1.CapQ468.accountAge(a.Id));
System.debug('Q4_67=' + gladetest1.CapQ467.accountAge(a.Id));
Collision__c c = new Collision__c(Name = 'S68-local', Marker__c = 'LOCAL');
insert c;
System.debug('K3_68=' + gladetest1.CapCollision68.run());
System.debug('K3_67=' + gladetest1.CapCollision67.run());
