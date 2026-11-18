trigger CovAccountTrigger on Account (after insert) {
    CovTriggerHandler.onAfterInsert(Trigger.new);
}
