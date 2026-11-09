trigger EvtSubTrigger on Evt__e (after insert) {
    Database.insert(new Obs__c(Name = 'o', Data__c = JSON.serialize(EvtSubProbe.inSubscriber())), AccessLevel.SYSTEM_MODE);
}
