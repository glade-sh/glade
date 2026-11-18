trigger SignalAfterCommitSubscriber on SignalAfterCommit__e (after insert) {
 PeSubscriber.receive('ac', Trigger.new);
}
