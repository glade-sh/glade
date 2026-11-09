trigger BeEvtITrig on EvtI__e (after insert) {
    for (EvtI__e e : Trigger.new) { BeProbe.onEvent('pei', e.Note__c); }
}
