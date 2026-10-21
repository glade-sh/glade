@IsTest
private class ShareDeleteObservation {
    @IsTest static void collect() {
        Map<String,Object> observed = new Map<String,Object>();
        List<FeatureParcel__c> records = new List<FeatureParcel__c>{new FeatureParcel__c(Name='one'),new FeatureParcel__c(Name='two')};
        insert records;
        observed.put('insertRows',Limits.getDmlRows());
        observed.put('ownerShares',[SELECT COUNT() FROM FeatureParcel__Share WHERE ParentId IN :records AND RowCause='Owner']);
        delete records;
        observed.put('deleteRows',Limits.getDmlRows());
        observed.put('deletedCount',[SELECT COUNT() FROM FeatureParcel__c WHERE Id IN :records AND IsDeleted=true ALL ROWS]);
        undelete records;
        observed.put('undeleteRows',Limits.getDmlRows());
        observed.put('restoredCount',[SELECT COUNT() FROM FeatureParcel__c WHERE Id IN :records]);
        delete records[0];
        List<Database.DeleteResult> results = Database.delete(records,false);
        observed.put('partialRows',Limits.getDmlRows());
        observed.put('partialSuccess',new List<Boolean>{results[0].isSuccess(),results[1].isSuccess()});
        Database.emptyRecycleBin(records);
        observed.put('purgeRows',Limits.getDmlRows());
        observed.put('purgedTestCount',[SELECT COUNT() FROM FeatureParcel__c WHERE Id IN :records ALL ROWS]);
        observed.put('statements',Limits.getDmlStatements());
        System.assert(false,'SHARE_DELETE|'+JSON.serialize(observed));
    }
}
