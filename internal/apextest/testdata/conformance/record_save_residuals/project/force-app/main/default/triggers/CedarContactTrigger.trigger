trigger CedarContactTrigger on Contact (before insert) {
    for (Contact c : Trigger.new) {
        if (c.LastName == 'TrgBlocked') { c.addError('contact trigger fired'); }
    }
}
