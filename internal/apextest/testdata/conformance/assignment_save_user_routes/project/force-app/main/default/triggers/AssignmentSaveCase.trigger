trigger AssignmentSaveCase on Case (before update, after update) {
    AssignmentSaveProbe.observe(Trigger.new, Trigger.oldMap, Trigger.isBefore);
}
