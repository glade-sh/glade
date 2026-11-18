delete [SELECT Id FROM Ord__c];
insert new List<Ord__c>{OrdSeed.row(7), OrdSeed.row(2), OrdSeed.row(11), OrdSeed.row(4)};
System.debug('OBS|seed1|n=' + [SELECT COUNT() FROM Ord__c]);
