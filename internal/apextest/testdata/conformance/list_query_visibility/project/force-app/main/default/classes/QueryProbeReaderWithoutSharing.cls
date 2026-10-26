public without sharing class QueryProbeReaderWithoutSharing {
    public static Integer countSetupRows() {
        return [SELECT COUNT() FROM QueryProbe__c WHERE Name = 'Setup'];
    }
}
