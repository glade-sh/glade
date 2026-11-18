trigger FernCaseTrace on Case (before insert, after insert, before update, after update) {
    FernCaseLog.events.add((Trigger.isBefore ? 'B' : 'A') + (Trigger.isInsert ? 'I' : 'U') + ':' + Trigger.size);
}
