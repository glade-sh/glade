trigger StopOrderContactTrigger on Contact (after insert) {
    System.enqueueJob(new StopOrderQ('tq', false));
}
