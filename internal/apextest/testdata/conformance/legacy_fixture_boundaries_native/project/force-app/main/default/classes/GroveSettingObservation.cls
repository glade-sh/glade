@IsTest
private class GroveSettingObservation {
    @IsTest static void updateAbsentDefault() {
        GroveOptions__c option = GroveOptions__c.getOrgDefaults();
        Map<String, Object> result = new Map<String, Object>{
            'initialNull' => option == null,
            'initialHasId' => option != null && option.Id != null
        };
        Boolean inserted = false;
        try {
            if (option == null) {
                option = new GroveOptions__c(SetupOwnerId = UserInfo.getOrganizationId());
                insert option;
                inserted = true;
            }
            option.Enabled__c = true;
            update option;
            result.put('updated', true);
        } catch (Exception fault) {
            result.put('updated', false);
            result.put('exceptionType', fault.getTypeName());
            result.put('exceptionMessage', fault.getMessage());
        }
        result.put('inserted', inserted);
        System.assert(false, 'OBSERVED|updateAbsentDefault|' + JSON.serialize(result));
    }

    @IsTest static void upsertAbsentDefault() {
        GroveOptions__c option = GroveOptions__c.getOrgDefaults();
        if (option == null) option = new GroveOptions__c();
        option.SetupOwnerId = UserInfo.getOrganizationId();
        option.Enabled__c = true;
        upsert option;
        System.assert(false, 'OBSERVED|upsertAbsentDefault|' + JSON.serialize(new Map<String, Object>{
            'hasId' => option.Id != null,
            'enabled' => GroveOptions__c.getOrgDefaults().Enabled__c
        }));
    }
}
