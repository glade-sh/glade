@IsTest private class OwnedFlowProjectTest {
    // Local caller-input variations: uncaptured: needs native row.
    @IsTest static void uncapturedChangedBeforeUpdate() {
        OwnedFlowHandler.stage = 'disabled';
        Account original = new Account(Name='Before', Description='Keep', Industry='Technology');
        insert original;
        OwnedFlowHandler.stage = 'beforeUpdate';
        original.Name = 'Changed';
        update original;
        Account stored = [SELECT Name, Description FROM Account WHERE Id=:original.Id];
        System.assertEquals('Foo', stored.Name, 'uncaptured: needs native row');
        System.assertEquals(null, stored.Description, 'uncaptured: needs native row');
    }
    @IsTest static void uncapturedDistinctAfterInsert() {
        OwnedFlowHandler.stage = 'afterInsert';
        insert new Account(Name='Other', Description='Keep');
        System.assertEquals(true, OwnedFlowHandler.returnedSuccess, 'uncaptured: needs native row');
        System.assertEquals('Foo', OwnedFlowHandler.returnedName, 'uncaptured: needs native row');
        System.assertEquals(null, OwnedFlowHandler.returnedDescription, 'uncaptured: needs native row');
        System.assertEquals(null, OwnedFlowHandler.returnedErrorMessage, 'uncaptured: needs native row');
    }
}
