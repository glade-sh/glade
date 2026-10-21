@IsTest private class SetupMetadataTest {
    @IsTest static void businessHours() {
        List<BusinessHours> records = [SELECT Name, IsActive, IsDefault, TimeZoneSidKey, MondayStartTime, MondayEndTime FROM BusinessHours WHERE Name='Grove Weekdays'];
        Map<String,Object> observed = new Map<String,Object>{'count'=>records.size()};
        if (!records.isEmpty()) {
            BusinessHours row = records[0];
            observed.put('name',row.Name); observed.put('active',row.IsActive); observed.put('default',row.IsDefault); observed.put('zone',row.TimeZoneSidKey);
            observed.put('mondayStart',String.valueOf(row.MondayStartTime)); observed.put('mondayEnd',String.valueOf(row.MondayEndTime));
        }
        System.assert(false,'OBSERVED|businessHours|'+JSON.serialize(observed));
    }
    @IsTest static void duplicateRule() {
        List<DuplicateRule> records = [SELECT DeveloperName, MasterLabel, IsActive, SobjectType FROM DuplicateRule WHERE DeveloperName='Grove_Email_Check'];
        Map<String,Object> observed = new Map<String,Object>{'count'=>records.size()};
        if (!records.isEmpty()) {
            DuplicateRule row=records[0]; observed.put('name',row.DeveloperName); observed.put('label',row.MasterLabel); observed.put('active',row.IsActive); observed.put('object',row.SobjectType);
        }
        System.assert(false,'OBSERVED|duplicateRule|'+JSON.serialize(observed));
    }
    @IsTest static void personDuplicateRule() {
        List<DuplicateRule> records = [SELECT DeveloperName, MasterLabel, IsActive, SobjectType, SobjectSubtype FROM DuplicateRule WHERE DeveloperName='Grove_PersonEmail_Check'];
        Map<String,Object> observed = new Map<String,Object>{'count'=>records.size()};
        if (!records.isEmpty()) {
            DuplicateRule row=records[0]; observed.put('name',row.DeveloperName); observed.put('label',row.MasterLabel); observed.put('active',row.IsActive); observed.put('object',row.SobjectType); observed.put('subtype',row.SobjectSubtype);
        }
        System.assert(false,'OBSERVED|personDuplicateRule|'+JSON.serialize(observed));
    }
}
