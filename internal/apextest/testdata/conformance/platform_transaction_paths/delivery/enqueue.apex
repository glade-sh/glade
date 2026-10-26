delete [SELECT Id FROM CaptureRecord__c];
delete [SELECT Id FROM CaptureLog__c];
insert new CaptureRecord__c(Name='async');
insert new CaptureRecord__c(Name='scheduled',NextRun__c=System.now().addMinutes(-1));
insert new CaptureRecord__c(Name='scheduled',NextRun__c=System.now().addDays(2));
Database.SaveResult published=EventBus.publish(new CapturePulse__e(Key__c='accept',RunKey__c='committed'));
System.assert(published.isSuccess());
