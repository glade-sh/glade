trigger CloneParcelTrigger on CloneParcel__c (after insert, after delete) {
    if (Trigger.isDelete) CloneWriteProbe.afterRows(Trigger.old,Trigger.oldMap);
    else CloneWriteProbe.afterRows(Trigger.new,Trigger.newMap);
}
