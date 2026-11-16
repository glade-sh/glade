Collision__c c = new Collision__c(Name = 'S68-local', Marker__c = 'LOCAL');
insert c;
System.debug('LOCAL_ROWS=' + [SELECT COUNT() FROM Collision__c]);
System.debug('K3_68=' + gladetest1.CapCollision68.run());
System.debug('K3_67=' + gladetest1.CapCollision67.run());
