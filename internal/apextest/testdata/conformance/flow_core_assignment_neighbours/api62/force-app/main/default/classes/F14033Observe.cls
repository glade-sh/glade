@IsTest private class F14033Observe { @IsTest static void observe() {
String unitName='F14_033'; Map<String,Object> payload=new Map<String,Object>();
FlowCoreF14Record__c subject=new FlowCoreF14Record__c(Name='subject', Unit__c=unitName, Ratio__c=10.75); insert subject;
subject=[SELECT Ratio__c FROM FlowCoreF14Record__c WHERE Id=:subject.Id]; payload.put('value',subject.Ratio__c);
System.assert(false,'FLOW_CORE_RESULT|F14_033|'+JSON.serialize(payload));
} }
