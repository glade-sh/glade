trigger LogSub on Log__e (after insert) {
    List<LogRec__c> recs = new List<LogRec__c>();
    for (Log__e e : Trigger.new) {
        recs.add(new LogRec__c(Usr__c = e.CreatedById, CB__c = String.valueOf(e.CreatedById), CD__c = e.CreatedDate));
    }
    insert recs;
}
