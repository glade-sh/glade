trigger GroveFormulaBeforeInsert on Account (before insert) {
    for (Account row : Trigger.new) {
        row.Description = JSON.serialize(new Map<String, Object>{
            'value' => row.GroveAge__c,
            'positive' => row.GroveAge__c > 0
        });
    }
}
