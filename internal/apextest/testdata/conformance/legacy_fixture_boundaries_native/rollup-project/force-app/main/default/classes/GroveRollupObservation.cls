@IsTest
private class GroveRollupObservation {
    private static Decimal disabledCount(Id groupId) {
        return [SELECT Disabled_Count__c FROM GroveGroup__c WHERE Id = :groupId WITH SYSTEM_MODE].Disabled_Count__c;
    }

    @IsTest static void checkboxNotEqualCount() {
        GroveGroup__c groupRow = new GroveGroup__c(Name = 'Group');
        Database.insert(groupRow, AccessLevel.SYSTEM_MODE);
        Map<String, Object> result = new Map<String, Object>{'empty' => disabledCount(groupRow.Id)};
        GroveLeaf__c offLeaf = new GroveLeaf__c(Name = 'Off', Group__c = groupRow.Id, Enabled__c = false);
        GroveLeaf__c onLeaf = new GroveLeaf__c(Name = 'On', Group__c = groupRow.Id, Enabled__c = true);
        GroveLeaf__c omittedLeaf = new GroveLeaf__c(Name = 'Omitted', Group__c = groupRow.Id);
        Database.insert(new List<GroveLeaf__c>{offLeaf, onLeaf, omittedLeaf}, AccessLevel.SYSTEM_MODE);
        result.put('afterInsert', disabledCount(groupRow.Id));
        result.put('omittedEnabled', [SELECT Enabled__c FROM GroveLeaf__c WHERE Id = :omittedLeaf.Id WITH SYSTEM_MODE].Enabled__c);
        offLeaf.Enabled__c = true;
        Database.update(offLeaf, AccessLevel.SYSTEM_MODE);
        result.put('afterUpdate', disabledCount(groupRow.Id));
        Database.delete(omittedLeaf, AccessLevel.SYSTEM_MODE);
        result.put('afterDelete', disabledCount(groupRow.Id));
        System.assert(false, 'OBSERVED|checkboxNotEqualCount|' + JSON.serialize(result));
    }
}
