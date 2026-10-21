trigger ImmediateSignalTrigger on ImmediateSignal__e (after insert) {TransactionStaticState.receive();}
