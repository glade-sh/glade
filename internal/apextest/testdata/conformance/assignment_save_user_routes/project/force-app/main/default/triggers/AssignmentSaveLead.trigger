trigger AssignmentSaveLead on Lead (before update, after update) {
    AssignmentSaveProbe.observe(Trigger.new, Trigger.oldMap, Trigger.isBefore);
}
