String unitName='F14_034'; Map<String,Object> payload=new Map<String,Object>();
FlowCoreF14Record__c subject=new FlowCoreF14Record__c(Name='subject', Unit__c=unitName); insert subject;
subject=[SELECT Amount__c FROM FlowCoreF14Record__c WHERE Id=:subject.Id]; payload.put('value',subject.Amount__c);
System.assert(false,'FLOW_CORE_RESULT|F14_034|'+JSON.serialize(payload));
