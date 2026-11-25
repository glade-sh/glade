trigger EvtTrigger on Evt__e (after insert) {
    for (Evt__e e : Trigger.new) {
        Integer parentCount = [SELECT COUNT() FROM Doc__c WHERE Id = :e.DocId__c WITH USER_MODE];
        Integer rsn = [SELECT COUNT() FROM Doc__Share WHERE RowCause IN ('Rsn__c') AND ParentId IN (:e.DocId__c) WITH USER_MODE];
        Integer all = [SELECT COUNT() FROM Doc__Share WITH USER_MODE];
        insert new Res__c(Name = 'parent' + parentCount + ';rsn' + rsn + ';all' + all);
    }
}
