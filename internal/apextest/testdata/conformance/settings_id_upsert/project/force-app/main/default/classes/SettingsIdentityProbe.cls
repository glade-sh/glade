public class SettingsIdentityProbe {
    public static String sample() {
        delete [SELECT Id FROM ProbePreference__c];
        ProbePreference__c pref = new ProbePreference__c(SetupOwnerId = UserInfo.getOrganizationId(), Value__c = 'first');
        upsert pref ProbePreference__c.Id;
        Id original = pref.Id;
        pref.Value__c = 'second';
        List<ProbePreference__c> entries = new List<ProbePreference__c>{pref};
        upsert entries ProbePreference__c.Id;
        pref.Value__c = 'third';
        Database.UpsertResult result = Database.upsert(pref, ProbePreference__c.Id, true);
        ProbePreference__c stored = [SELECT Id, Value__c FROM ProbePreference__c WHERE Id = :original];
        return JSON.serialize(new Map<String, Object>{'sameId' => stored.Id == original, 'value' => stored.Value__c, 'success' => result.isSuccess(), 'created' => result.isCreated()});
    }
}
