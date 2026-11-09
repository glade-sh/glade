trigger BeEvtTrig on Evt__e (after insert) {
    for (Evt__e e : Trigger.new) { BeProbe.onEvent('pe', e.Note__c); }
}
