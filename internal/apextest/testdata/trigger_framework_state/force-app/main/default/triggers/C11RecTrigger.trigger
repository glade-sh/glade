trigger C11RecTrigger on C11Rec__c (before insert) {
    C11Handler.beforeInsert(Trigger.new);
}
