@IsTest private class F13010Observe { @IsTest static void observe() {
String unitName='F13_010'; Map<String,Object> payload=new Map<String,Object>();
List<FlowCoreF13Record__c> prior=[SELECT Id FROM FlowCoreF13Record__c WHERE Unit__c=:unitName]; if(!prior.isEmpty()) delete prior;
FlowCoreF13Record__c target=new FlowCoreF13Record__c(Name='target',Unit__c=unitName,Text__c='chosen',Whole__c=7,Flag__c=true); insert target;
FlowCoreF13Record__c subject=new FlowCoreF13Record__c(Name='subject',Unit__c=unitName,Gate__c=true); insert subject;
payload.put('remaining',[SELECT COUNT() FROM FlowCoreF13Record__c WHERE Unit__c=:unitName AND Name='target']);
payload.put('effects',[SELECT COUNT() FROM FlowCoreF13Record__c WHERE Unit__c=:unitName AND Name='log' AND Result__c='Deleted']);
System.assert(false,'FLOW_CORE_RESULT|F13_010|'+JSON.serialize(payload));
} }
