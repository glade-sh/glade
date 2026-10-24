trigger CdcItemObserver on CdcItem__ChangeEvent (after insert) {
    for (CdcItem__ChangeEvent event : Trigger.new) CdcGapObservation.record(event, event.ChangeEventHeader);
}
