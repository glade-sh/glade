String unitName='N3_052'; Map<String,Object> payload=new Map<String,Object>();
FlowCoreN3Record__c equalOnly=new FlowCoreN3Record__c(Name='subject',Unit__c=unitName,Gate__c=true,Amount__c=10.75, Whole__c=0); insert equalOnly; payload.put('equalOnly',[SELECT COUNT() FROM FlowCoreN3Record__c WHERE Unit__c=:unitName AND Name='log']);
FlowCoreN3Record__c greaterOnly=new FlowCoreN3Record__c(Name='subject',Unit__c=unitName,Gate__c=true,Amount__c=5, Whole__c=11); insert greaterOnly; payload.put('greaterOnly',[SELECT COUNT() FROM FlowCoreN3Record__c WHERE Unit__c=:unitName AND Name='log']);
FlowCoreN3Record__c neither=new FlowCoreN3Record__c(Name='subject',Unit__c=unitName,Gate__c=true,Amount__c=5, Whole__c=0); insert neither; payload.put('neither',[SELECT COUNT() FROM FlowCoreN3Record__c WHERE Unit__c=:unitName AND Name='log']);
System.assert(false,'FLOW_CORE_RESULT|F4F6_052|'+JSON.serialize(payload));
