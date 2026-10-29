public class RuntimeSettingCases {
    public static String missingCredential(String name) {
        try {
            RuntimeCredential__c entry = [SELECT Token__c FROM RuntimeCredential__c WHERE Name = :name LIMIT 1];
            return entry.Token__c;
        } catch (Exception e) {
            if (e.getMessage() == 'List has no rows for assignment to SObject') {
                throw new RuntimeBoundaryErrors.QueryException('missing credential');
            }
            throw e;
        }
    }
    public static Map<String, Object> observe() {
        delete [SELECT Id FROM RuntimeCredential__c];
        Map<String, Object> result = new Map<String, Object>();
        try { missingCredential('absent'); }
        catch (Exception e) { result.put('emptySetting', e.getTypeName() + '|' + e.getMessage()); }
        insert new RuntimeCredential__c(Name='other', Token__c='dummy');
        try { missingCredential('absent'); }
        catch (Exception e) { result.put('wrongSetting', e.getTypeName() + '|' + e.getMessage()); }
        insert new RuntimeCredential__c(Name='present', Token__c='local-value');
        result.put('presentSetting', missingCredential('present'));
        try { Database.query('SELECT Id FROM RuntimeMissingSetting__c'); }
        catch (Exception e) { result.put('missingSourceType', e.getTypeName()); }
        RuntimePreference__c preference = RuntimePreference__c.getOrgDefaults();
        result.put('emptyDefaultsIsNull', preference == null);
        result.put('emptyDefaultsIdIsNull', preference.Id == null);
        preference.Enabled__c = true;
        try { update preference; }
        catch (DmlException e) { result.put('emptyDefaultsUpdate', String.valueOf(e.getDmlStatusCode(0))); }
        List<RuntimeAccess__mdt> entries = [SELECT Token__c FROM RuntimeAccess__mdt WHERE Label = 'Sample'];
        result.put('metadataRows', entries.size());
        result.put('metadataValue', entries[0].Token__c);
        result.put('metadataGetterValue', RuntimeAccess__mdt.getInstance('Sample').Token__c);
        try {
            if (entries[0].Token__c == 'replace-me') throw new RuntimeBoundaryErrors.TokenException('placeholder token');
            result.put('metadataGuard', 'ready');
        } catch (Exception e) { result.put('metadataGuard', e.getMessage()); }
        result.put('missingMetadata', RuntimeAccess__mdt.getInstance('Absent') == null);
        result.put('syntheticUserRows', [SELECT count() FROM User WHERE Alias='zzleft']);
        return result;
    }
}
