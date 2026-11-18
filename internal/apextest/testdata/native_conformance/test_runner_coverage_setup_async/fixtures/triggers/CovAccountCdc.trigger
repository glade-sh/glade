trigger CovAccountCdc on AccountChangeEvent (after insert) {
    for (AccountChangeEvent e : Trigger.new) {
        insert new Contact(LastName = 'R3-CDC');
    }
}
