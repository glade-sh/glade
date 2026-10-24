trigger CedarHeaderObserver on AccountChangeEvent (after insert) {
    List<Contact> output = new List<Contact>();
    for (AccountChangeEvent eventRow : Trigger.new) {
        EventBus.ChangeEventHeader h = eventRow.ChangeEventHeader;
        Boolean selected = false;
        for (String recordId : h.recordIds) {
            if (CedarHeaderProbe.recordIds.contains(recordId)) selected = true;
        }
        if (!selected) continue;
        List<String> changed = new List<String>();
        List<String> nulled = new List<String>();
        // Audit timestamps may change between DML statements; observe business fields.
        for (String field : h.changedFields) {
            if (field == 'Name' || field == 'Description') changed.add(field);
        }
        for (String field : h.nulledFields) {
            if (field == 'Name' || field == 'Description') nulled.add(field);
        }
        changed.sort(); nulled.sort();
        String summary = h.entityName + '/' + h.changeType + '/' + h.recordIds.size()
            + '/commit=' + (h.commitNumber != null && h.commitNumber > 0)
            + '/user=' + (h.commitUser == CedarHeaderProbe.sourceUser)
            + '/seq=' + (h.sequenceNumber != null && h.sequenceNumber > 0)
            + '/time=' + (h.commitTimestamp != null && h.commitTimestamp > 0)
            + '/tx=' + !String.isBlank(h.transactionKey)
            + '/origin=' + !String.isBlank(h.changeOrigin)
            + '/changed=' + String.join(changed, ',')
            + '/nulled=' + String.join(nulled, ',')
            + '/diff=' + String.join(h.diffFields, ',');
        output.add(new Contact(LastName='cedar-header', Description=summary));
    }
    if (!output.isEmpty()) insert output;
}
