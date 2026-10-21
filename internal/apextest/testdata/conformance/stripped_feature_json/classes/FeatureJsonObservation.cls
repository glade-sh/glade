@IsTest
private class FeatureJsonObservation {
    @IsTest static void collect() {
        Profile profile = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        User actor = new User(ProfileId=profile.Id, Username='feature.json.'+UserInfo.getOrganizationId()+'@example.invalid', Email='feature@example.invalid', LastName='Feature', Alias='feature', TimeZoneSidKey='America/Los_Angeles', LocaleSidKey='en_US', EmailEncodingKey='UTF-8', LanguageLocaleKey='en_US');
        insert actor;
        String prefix = FeatureParcel__c.SObjectType.getDescribe().getKeyPrefix();
        FeatureParcel__c source = new FeatureParcel__c(Id=(Id)(prefix+'000000000001'), Name='Owned');
        FeatureParcel__c transientRecord = new FeatureParcel__c(Name='Owned');
        Map<String,Object> observed = new Map<String,Object>();
        System.runAs(actor) {
            List<SObject> clean = Security.stripInaccessible(AccessType.READABLE, new List<SObject>{source,transientRecord}, false).getRecords();
            observed.put('single', JSON.serializePretty(clean[0]));
            observed.put('list', JSON.serializePretty(clean));
            observed.put('map', JSON.serializePretty(new Map<String,SObject>{String.valueOf(source.Id)=>clean[0], ''=>clean[1]}));
            observed.put('blank', JSON.serializePretty(clean[1]));
            observed.put('sourceUnchanged', source.Name=='Owned');
            observed.put('singleEqualsFresh', JSON.serializePretty(clean[0])==JSON.serializePretty(new FeatureParcel__c(Id=source.Id)));
            observed.put('blankEqualsFresh', JSON.serializePretty(clean[1])==JSON.serializePretty(new FeatureParcel__c()));
            observed.put('objectAccessible', FeatureParcel__c.SObjectType.getDescribe().isAccessible());
        }
        System.assert(false, 'FEATURE_JSON|'+JSON.serialize(observed));
    }
}
