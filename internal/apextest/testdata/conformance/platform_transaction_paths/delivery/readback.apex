Datetime clock=System.now();
CaptureRecord__c asyncRow=[SELECT Immediate__c,Marker__c FROM CaptureRecord__c WHERE Name='async' LIMIT 1];
CaptureRecord__c dueRow=[SELECT Immediate__c,Marker__c FROM CaptureRecord__c WHERE Name='scheduled' AND NextRun__c<:clock LIMIT 1];
CaptureRecord__c futureRow=[SELECT Immediate__c,Marker__c FROM CaptureRecord__c WHERE Name='scheduled' AND NextRun__c>:clock LIMIT 1];
Map<String,Object> observations=new Map<String,Object>{
 'F07'=>new Map<String,Object>{'immediate'=>asyncRow.Immediate__c,'delayed'=>asyncRow.Marker__c},
 'F08'=>new Map<String,Object>{'immediate'=>dueRow.Immediate__c,'delayed'=>dueRow.Marker__c},
 'F09'=>new Map<String,Object>{'immediate'=>futureRow.Immediate__c,'delayed'=>futureRow.Marker__c},
 'P08'=>new Map<String,Object>{'count'=>[SELECT count() FROM CaptureLog__c WHERE Name='committed']}
};
System.debug('PLATFORM_DELIVERY|'+JSON.serialize(observations));
