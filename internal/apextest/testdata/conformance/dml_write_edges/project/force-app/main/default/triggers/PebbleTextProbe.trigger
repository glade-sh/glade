trigger PebbleTextProbe on Pebble__c (before insert, before update) {
    for (Pebble__c row : Trigger.new) {
        row.Observed__c = DmlWriteProbe.frame(row.Name) + '|' + DmlWriteProbe.frame(row.Short__c) + '|' +
            DmlWriteProbe.frame(row.Note__c) + '|' + DmlWriteProbe.frame(row.LongNote__c);
    }
}
