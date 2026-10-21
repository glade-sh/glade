@IsTest
private class DeletedLookupObservationTest {
    @IsTest static void retainedChildUpdate() { observe('retainedChildUpdate',true); }
    @IsTest static void sparseChildUpdate() { observe('sparseChildUpdate',false); }
    @IsTest static void freshChildLookupUpdate() { observeBoundary('freshChildLookupUpdate','fresh'); }
    @IsTest static void newChildInsert() { observeBoundary('newChildInsert','insert'); }
    @IsTest static void differentDeletedParentUpdate() { observeBoundary('differentDeletedParentUpdate','different'); }
    @IsTest static void explicitNullThenRestore() { observeReset('explicitNullThenRestore',false); }
    @IsTest static void rolledBackNullThenRestore() { observeReset('rolledBackNullThenRestore',true); }
    private static void observe(String method,Boolean retained) {
        ParentMemo__c parent = new ParentMemo__c(Name='Parent');
        insert parent;
        ChildMemo__c child = new ChildMemo__c(Name='Child',Parent__c=parent.Id);
        insert child;
        delete parent;
        List<ChildMemo__c> afterDelete = [SELECT Id,Parent__c FROM ChildMemo__c WHERE Id=:child.Id];
        Map<String,Object> observation = new Map<String,Object>{'survivingChildren'=>afterDelete.size()};
        if (!afterDelete.isEmpty()) observation.put('lookupCleared',afterDelete[0].Parent__c==null);
        ChildMemo__c target = retained ? child : new ChildMemo__c(Id=child.Id);
        target.Name = 'After update';
        Database.SaveResult result = Database.update(target,false);
        observation.put('updateSuccess',result.isSuccess());
        if (!result.isSuccess()) observation.put('updateStatus',String.valueOf(result.getErrors()[0].getStatusCode()));
        List<ChildMemo__c> afterUpdate = [SELECT Name,Parent__c FROM ChildMemo__c WHERE Id=:child.Id];
        if (!afterUpdate.isEmpty()) {
            observation.put('afterName',afterUpdate[0].Name);
            observation.put('afterLookupCleared',afterUpdate[0].Parent__c==null);
        }
        System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(observation));
    }

    private static void observeBoundary(String method,String mode) {
        ParentMemo__c originalParent = new ParentMemo__c(Name='Original parent');
        insert originalParent;
        ChildMemo__c originalChild = new ChildMemo__c(Name='Original child',Parent__c=originalParent.Id);
        insert originalChild;
        ParentMemo__c deletedParent = originalParent;
        if (mode=='different') {
            deletedParent = new ParentMemo__c(Name='Different parent');
            insert deletedParent;
        }
        delete deletedParent;
        ChildMemo__c target = new ChildMemo__c(Name='After update',Parent__c=deletedParent.Id);
        if (mode!='insert') target.Id=originalChild.Id;
        Database.SaveResult result;
        if (mode=='insert') result=Database.insert(target,false);
        else result=Database.update(target,false);
        Map<String,Object> observation = new Map<String,Object>{'success'=>result.isSuccess()};
        if (!result.isSuccess()) observation.put('status',String.valueOf(result.getErrors()[0].getStatusCode()));
        List<ChildMemo__c> stored = [SELECT Name,Parent__c FROM ChildMemo__c WHERE Id=:target.Id];
        observation.put('storedCount',stored.size());
        if (!stored.isEmpty()) {
            observation.put('storedName',stored[0].Name);
            observation.put('lookupCleared',stored[0].Parent__c==null);
            observation.put('matchesDeletedParent',stored[0].Parent__c==deletedParent.Id);
            observation.put('matchesOriginalParent',stored[0].Parent__c==originalParent.Id);
        }
        System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(observation));
    }

    private static void observeReset(String method,Boolean rollbackNull) {
        ParentMemo__c parent = new ParentMemo__c(Name='Reset parent');
        insert parent;
        ChildMemo__c child = new ChildMemo__c(Name='Reset child',Parent__c=parent.Id);
        insert child;
        delete parent;
        Savepoint checkpoint;
        if (rollbackNull) checkpoint=Database.setSavepoint();
        Database.SaveResult cleared=Database.update(new ChildMemo__c(Id=child.Id,Parent__c=null),false);
        Map<String,Object> observation=new Map<String,Object>{'clearSuccess'=>cleared.isSuccess()};
        if (!cleared.isSuccess()) observation.put('clearStatus',String.valueOf(cleared.getErrors()[0].getStatusCode()));
        if (rollbackNull) Database.rollback(checkpoint);
        Database.SaveResult restored=Database.update(new ChildMemo__c(Id=child.Id,Name='Restored child',Parent__c=parent.Id),false);
        observation.put('restoreSuccess',restored.isSuccess());
        if (!restored.isSuccess()) observation.put('restoreStatus',String.valueOf(restored.getErrors()[0].getStatusCode()));
        ChildMemo__c stored=[SELECT Name,Parent__c FROM ChildMemo__c WHERE Id=:child.Id];
        observation.put('storedName',stored.Name);
        observation.put('lookupCleared',stored.Parent__c==null);
        observation.put('matchesDeletedParent',stored.Parent__c==parent.Id);
        System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(observation));
    }
}
