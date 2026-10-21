trigger OrchardJournal on JournalEntry__c (before insert, after insert) {
    if (Trigger.isBefore) {
        for (JournalEntry__c entry : Trigger.new) entry.Details__c = 'trigger: ' + entry.Details__c;
    } else {
        OrchardHistoryTest.created += Trigger.new.size();
    }
}
