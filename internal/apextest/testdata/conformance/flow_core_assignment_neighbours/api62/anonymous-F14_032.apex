String unitName='F14_032'; Map<String,Object> payload=new Map<String,Object>();
FlowCoreF14Record__c subject=new FlowCoreF14Record__c(Name='subject', Unit__c=unitName, Money__c=10.75); insert subject;
subject=[SELECT Money__c FROM FlowCoreF14Record__c WHERE Id=:subject.Id]; payload.put('value',subject.Money__c);
System.assert(false,'FLOW_CORE_RESULT|F14_032|'+JSON.serialize(payload));
