trigger CdcAccountObserver on AccountChangeEvent (after insert) {
    for (AccountChangeEvent event : Trigger.new) CdcGapObservation.record(event, event.ChangeEventHeader);
}
