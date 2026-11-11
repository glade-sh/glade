trigger EvtSubTrigger on Evt__e (after insert) {
    APProbe.record('trigger');
    System.enqueueJob(new APProbe.Q('queueable_from_trigger'));
    APProbe.fut('future_from_trigger');
}
