@IsTest private class F4F6039Observe { @IsTest static void observe() {
String unitName='N3_039'; Map<String,Object> payload=new Map<String,Object>();
FlowCoreN3Record__c subject=new FlowCoreN3Record__c(Name='subject', Unit__c='N3_039', Gate__c=true, Text__c='seed', Trace__c='seed', Other__c='seed', Amount__c=10.75, Whole__c=0); insert subject;
subject.Text__c='Beta'; subject.Trace__c='reset'; subject.Other__c='updated'; update subject;
subject=[SELECT Trace__c FROM FlowCoreN3Record__c WHERE Id=:subject.Id]; Integer effects=[SELECT COUNT() FROM FlowCoreN3Record__c WHERE Unit__c=:unitName AND Name='log'];
payload.put('traceAfterInsert','seed'); payload.put('traceAfterUpdate',subject.Trace__c); payload.put('effects',effects);
System.assert(false,'FLOW_CORE_RESULT|F4F6_039|'+JSON.serialize(payload));
} }
