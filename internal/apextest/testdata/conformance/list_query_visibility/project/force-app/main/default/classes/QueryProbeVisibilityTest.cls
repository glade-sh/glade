@IsTest public with sharing class QueryProbeVisibilityTest {
    @TestSetup static void seed() {
        insert new QueryProbe__c(Name = 'Setup');
    }

    @IsTest static void visibility() {
        Map<String, Object> row = new Map<String, Object>();
        Profile profile = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        User actor = new User(Username = 'query.probe.' + UserInfo.getOrganizationId() + '@example.invalid', Alias = 'qprobe', Email = 'query.probe@example.invalid', LastName = 'Query Probe Reader', ProfileId = profile.Id, TimeZoneSidKey = 'America/Los_Angeles', LocaleSidKey = 'en_US', EmailEncodingKey = 'UTF-8', LanguageLocaleKey = 'en_US');
        insert actor;
        PermissionSet permissionSet = [SELECT Id FROM PermissionSet WHERE Name = 'QueryProbeReader' LIMIT 1];
        insert new PermissionSetAssignment(AssigneeId = actor.Id, PermissionSetId = permissionSet.Id);

        row.put('setupCount', [SELECT COUNT() FROM QueryProbe__c WHERE Name = 'Setup']);
        System.runAs(actor) {
            RecordType alpha = [SELECT Id FROM RecordType WHERE SObjectType = 'QueryProbe__c' AND DeveloperName = 'Alpha' LIMIT 1];
            RecordType beta = [SELECT Id FROM RecordType WHERE SObjectType = 'QueryProbe__c' AND DeveloperName = 'Beta' LIMIT 1];
            insert new List<QueryProbe__c>{
                new QueryProbe__c(Name = 'Alpha row', RecordTypeId = alpha.Id),
                new QueryProbe__c(Name = 'Beta row', RecordTypeId = beta.Id)
            };
            row.put('alphaCount', [SELECT COUNT() FROM QueryProbe__c WHERE RecordType.DeveloperName = 'Alpha']);
            row.put('betaCount', [SELECT COUNT() FROM QueryProbe__c WHERE RecordType.DeveloperName = 'Beta']);
            row.put('methodCount', [SELECT COUNT() FROM QueryProbe__c WHERE Name = 'Alpha row']);
            row.put('withSharingCount', QueryProbeReaderWithSharing.countSetupRows());
            row.put('withoutSharingCount', QueryProbeReaderWithoutSharing.countSetupRows());
            try {
                QueryProbe__c visible = [SELECT Id FROM QueryProbe__c WHERE Name = 'Setup' LIMIT 1];
                row.put('withSharingSingleton', visible.Id != null);
            } catch (Exception error) {
                row.put('withSharingSingletonType', error.getTypeName());
                row.put('withSharingSingletonMessage', error.getMessage());
            }
        }

        try {
            QueryProbe__c missing = [SELECT Id FROM QueryProbe__c WHERE Name = 'Missing' LIMIT 1];
            row.put('emptySingletonType', 'returned');
            row.put('emptySingletonId', missing.Id);
        } catch (Exception error) {
            row.put('emptySingletonType', error.getTypeName());
            row.put('emptySingletonMessage', error.getMessage());
        }
        row.put('seeAllDataFalseCount', [SELECT COUNT() FROM Account WHERE Name = 'Glade Synthetic Existing Row']);
        System.assert(false, 'OBSERVED|visibility|' + JSON.serialize(row));
    }
}
