@IsTest private class FlowCoreN2ObservationTest {
    private static Map<String, Object> read(Id recordId) {
        FlowCoreN2Record__c row = [SELECT Text__c, UpdateMark__c FROM FlowCoreN2Record__c WHERE Id=:recordId];
        return new Map<String, Object>{'text' => row.Text__c, 'updateMark' => row.UpdateMark__c};
    }
    @IsTest static void observe() {
        String unitName = 'N2' + String.valueOf(DateTime.now().getTime());
        Map<String, Object> payload = new Map<String, Object>();
        FlowCoreN2Record__c inserted = new FlowCoreN2Record__c(Name='insert', Unit__c=unitName, ExternalKey__c=unitName+'-insert', Text__c='seed');
        insert inserted;
        payload.put('insert', read(inserted.Id));
        inserted.Text__c='before-update'; inserted.UpdateMark__c='seed-update'; update inserted;
        payload.put('update', read(inserted.Id));
        FlowCoreN2Record__c externalInsert = new FlowCoreN2Record__c(Name='external-insert', Unit__c=unitName, ExternalKey__c=unitName+'-external-insert', Text__c='seed');
        upsert externalInsert ExternalKey__c;
        payload.put('upsertExternalInsert', read(externalInsert.Id));
        FlowCoreN2Record__c externalSeed = new FlowCoreN2Record__c(Name='external-update', Unit__c=unitName, ExternalKey__c=unitName+'-external-update', Text__c='seed');
        insert externalSeed;
        FlowCoreN2Record__c externalUpdate = new FlowCoreN2Record__c(Name='external-update', Unit__c=unitName, ExternalKey__c=externalSeed.ExternalKey__c, Text__c='before-external-upsert', UpdateMark__c='seed-update');
        upsert externalUpdate ExternalKey__c;
        payload.put('upsertExternalUpdate', read(externalUpdate.Id));
        FlowCoreN2Record__c idSeed = new FlowCoreN2Record__c(Name='id-update', Unit__c=unitName, ExternalKey__c=unitName+'-id-update', Text__c='seed');
        insert idSeed;
        FlowCoreN2Record__c idUpdate = new FlowCoreN2Record__c(Id=idSeed.Id, Name='id-update', Unit__c=unitName, ExternalKey__c=idSeed.ExternalKey__c, Text__c='before-id-upsert', UpdateMark__c='seed-update');
        upsert idUpdate;
        payload.put('upsertIdUpdate', read(idSeed.Id));
        System.assert(false, 'FLOW_CORE_RESULT|N2_001|' + JSON.serialize(payload));
    }
}
