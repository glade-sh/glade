trigger SignalImmediateSubscriber on SignalImmediate__e (after insert) {
 PeSubscriber.receive('imm', Trigger.new);
}
