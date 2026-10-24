trigger AssignmentSaveLead on Lead (before insert, after insert, before update, after update) {
    AssignmentSaveProbe.observe(Trigger.new, Trigger.oldMap, Trigger.isBefore);
}
