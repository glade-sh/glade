public without sharing class ProbeSettingsService {
    public static Map<String,Object> lastCaught;

    public static String getValue(String wantedName, String wantedKey) {
        ProbeCredential__c credential;
        lastCaught = null;
        try {
            credential = [
                SELECT Id, Name, ProbeKey__c, ProbeValue__c
                FROM ProbeCredential__c
                WHERE Name = :wantedName AND ProbeKey__c = :wantedKey
                LIMIT 1
            ];
        } catch (Exception ex) {
            lastCaught = new Map<String,Object>{'type'=>ex.getTypeName(), 'message'=>ex.getMessage(), 'line'=>ex.getLineNumber()};
            if (ex.getMessage() == 'List has no rows for assignment to SObject') {
                throw new ProbeErrors.QueryException('Missing ' + wantedName + ' custom setting, please ensure this is set up before proceeding.');
            }
            throw ex;
        }
        return credential.ProbeValue__c;
    }
}
