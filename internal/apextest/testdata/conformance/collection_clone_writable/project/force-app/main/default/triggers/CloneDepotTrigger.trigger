trigger CloneDepotTrigger on CloneDepot__c (after insert) {
    if (CloneWriteProbe.enabled) CloneWriteProbe.drive(Trigger.new[0].Id,CloneWriteProbe.operation,CloneWriteProbe.shape);
}
