trigger CovEvtTrigger on CovEvt__e (after insert) {
    for (CovEvt__e e : Trigger.new) {
        insert new Account(Name = 'R3-EVENT');
    }
}
