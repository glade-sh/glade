trigger ElementTrigger on ElementSignal__e (after insert) {
 for(ElementSignal__e signal:Trigger.new) {
  if(signal.Operation__c=='order') insert new ElementLog__c(Name=signal.Key__c,Phase__c='trigger',Actor__c=UserInfo.getUserId());
 }
}
