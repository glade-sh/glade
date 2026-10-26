public with sharing class QueryProbeReaderWithSharing {
    public static Integer countSetupRows() {
        return [SELECT COUNT() FROM QueryProbe__c WHERE Name = 'Setup'];
    }
}
