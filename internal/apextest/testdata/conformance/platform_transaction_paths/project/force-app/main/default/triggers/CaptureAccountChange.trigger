trigger CaptureAccountChange on AccountChangeEvent (after insert) {
 CapturePlatformProbe.cdcCount += Trigger.new.size();
 insert new CaptureLog__c(Name='cdc');
}
