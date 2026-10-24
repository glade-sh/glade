trigger CdcContactObserver on ContactChangeEvent (after insert) {
    for (ContactChangeEvent event : Trigger.new) CdcGapObservation.record(event, event.ChangeEventHeader);
}
