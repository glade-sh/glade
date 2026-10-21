trigger RelationshipWriteTrigger on Contact (before insert, after insert) {
    if (RelationshipWriteProbe.phase == 'before' && Trigger.isBefore) {
        RelationshipWriteProbe.attempt(Trigger.new[0], 'before');
    }
    if (RelationshipWriteProbe.phase == 'after' && Trigger.isAfter) {
        RelationshipWriteProbe.attempt(Trigger.new[0], 'after');
    }
}
