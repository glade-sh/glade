String unitName='N3_040'; Map<String,Object> payload=new Map<String,Object>();
FlowCoreN3Record__c subject=new FlowCoreN3Record__c(Name='subject', Unit__c='N3_040', Gate__c=true, Text__c='AlphaTail', Trace__c='seed', Other__c='seed', Amount__c=10.75, Whole__c=0); insert subject;
subject=[SELECT Trace__c FROM FlowCoreN3Record__c WHERE Id=:subject.Id]; payload.put('traceAfterInsert',subject.Trace__c);
subject.Text__c='Beta'; subject.Trace__c='reset'; subject.Other__c='updated'; update subject;
subject=[SELECT Trace__c FROM FlowCoreN3Record__c WHERE Id=:subject.Id]; Integer effects=[SELECT COUNT() FROM FlowCoreN3Record__c WHERE Unit__c=:unitName AND Name='log']; payload.put('traceAfterUpdate',subject.Trace__c); payload.put('effects',effects);
System.assert(false,'FLOW_CORE_RESULT|F4F6_040|'+JSON.serialize(payload));
