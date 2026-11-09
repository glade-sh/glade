delete [SELECT Id FROM C4Obs__c];
EventBus.publish(new List<C4Evt__e>{new C4Evt__e(Note__c = 'npe'), new C4Evt__e(Note__c = 'limit')});
EventBus.publish(new C4Evt__e(Note__c = 'listx'));
insert new List<C4Cdc__c>{new C4Cdc__c(Name = 'npe')};
insert new C4Cdc__c(Name = 'limit');
System.debug('CAP|evt_anon_published|ok');
