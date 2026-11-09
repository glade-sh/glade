trigger CdcTrig on Cdc__ChangeEvent (after insert) {
    CdcProbe.record('cdc_trigger', null);
    System.enqueueJob(new CdcProbe.Q('cdc_queueable', true));
    CdcProbe.fut('cdc_future');
}
