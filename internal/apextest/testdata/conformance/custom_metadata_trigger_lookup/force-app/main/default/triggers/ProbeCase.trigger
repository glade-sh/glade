trigger ProbeCase on Case (before insert) {
 ProbeFlag__mdt config = ProbeFlag__mdt.getInstance('Case');
 for (Case c : Trigger.new) {
  if (config == null || !config.Active__c || config.ObjectName__c != 'Case') c.addError('metadata lookup failed');
 }
}
