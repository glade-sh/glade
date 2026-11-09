trigger C4EvtTrig on C4Evt__e (after insert) {
    for (C4Evt__e ev : Trigger.new) {
        C4Probe.run('pe', ev.Note__c);
    }
}
