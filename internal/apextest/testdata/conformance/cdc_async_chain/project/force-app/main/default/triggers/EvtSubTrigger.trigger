trigger EvtSubTrigger on Evt__e (after insert) {
    CdcProbe.record('event_trigger', null);
    System.enqueueJob(new CdcProbe.Q('event_queueable', true));
    CdcProbe.fut('event_future');
}
