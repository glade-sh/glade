insert new Gb__c(Name='g1', Ck__c=true, Cu__c=1.5, Pc__c=2.5, Nm__c=3.25, Tx__c='t');
List<AggregateResult> rs = Database.query('SELECT Pc__c g, COUNT(Id) c FROM Gb__c GROUP BY Pc__c');
System.debug('OBS|gb_Pc|ok' + rs.size());
