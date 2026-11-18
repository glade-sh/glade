trigger CvTrigMulti on Contact (before insert, after insert,
                                before update) {
    if (Trigger.isBefore) {
        Integer a = 1;
    }
    if (Trigger.isAfter) {
        Integer b = 2;
    }
}
