trigger CommittedSignalTrigger on CommittedSignal__e (after insert) {TransactionStaticState.receive();}
