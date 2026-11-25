trigger EvtTrigger on Evt__e (after insert) {
    for (Evt__e e : Trigger.new) {
        List<Doc__c> unrelated = [SELECT Id FROM Doc__c WHERE Name = 'other' WITH SYSTEM_MODE];
        Id otherId = unrelated[0].Id;
        Integer parentCount = [SELECT COUNT() FROM Doc__c WHERE Id = :e.DocId__c WITH USER_MODE];
        Integer otherCount = [SELECT COUNT() FROM Doc__c WHERE Id = :otherId WITH USER_MODE];
        Integer relatedShare = [SELECT COUNT() FROM Doc__Share WHERE RowCause IN ('Rsn__c') AND ParentId IN (:e.DocId__c) WITH USER_MODE];
        Integer otherShare = [SELECT COUNT() FROM Doc__Share WHERE RowCause IN ('Rsn__c') AND ParentId IN (:otherId) WITH USER_MODE];
        Integer allShares = [SELECT COUNT() FROM Doc__Share WITH USER_MODE];
        insert new Res__c(Name = 'parent' + parentCount + ';other' + otherCount + ';related' + relatedShare + ';unrelated' + otherShare + ';all' + allShares);
    }
}
