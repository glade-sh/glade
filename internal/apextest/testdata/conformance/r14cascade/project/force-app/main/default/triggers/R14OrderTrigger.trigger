trigger R14OrderTrigger on R14Order__c (after insert) {
    System.assertEquals(2, Trigger.newMap.size());
    for (R14Order__c row : Trigger.newMap.values()) {
        System.assertNotEquals(null, row);
        System.assertEquals(row.Name == 'explicit', row.Flag__c);
        System.assertEquals(row.Name != 'explicit', !row.Flag__c);
        if (row.Name == 'default') R14Driver.defaultSeen++;
        if (row.Name == 'explicit') R14Driver.explicitSeen++;
    }
}
