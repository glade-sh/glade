trigger UpdateMessageTrigger on Contact (before update) {
    for (Contact row : Trigger.new) {
        if (row.Title == 'reject-trigger') row.addError('Synthetic trigger rejection');
        if (row.Title == 'reject-field') row.FirstName.addError('Synthetic field rejection');
    }
}
