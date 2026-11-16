insert new Gb__c(Name='g1', Ck__c=true, Cu__c=1.5, Pc__c=2.5, Nm__c=3.25, Tx__c='t');
try { Database.query('SELECT Nm__c g, COUNT(Id) c FROM Gb__c GROUP BY Nm__c'); System.debug('OBS|gbc2|ok'); }
catch (System.UnexpectedException e) { System.debug('OBS|gbc2|caught-Unexpected'); }
System.debug('OBS|gbc2|after');