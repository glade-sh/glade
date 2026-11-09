public with sharing class IspSharingReader {
    public static Integer count(String objectName) {
        if (objectName == 'IspJ__c') return [SELECT Id FROM IspJ__c WITH SYSTEM_MODE ORDER BY Id].size();
        if (objectName == 'IspP1__c') return [SELECT Id FROM IspP1__c WITH SYSTEM_MODE ORDER BY Id].size();
        return [SELECT Id FROM IspP2__c WITH SYSTEM_MODE ORDER BY Id].size();
    }
    public static Integer countUserMode(String objectName) {
        if (objectName == 'IspJ__c') return [SELECT Id FROM IspJ__c WITH USER_MODE ORDER BY Id].size();
        if (objectName == 'IspP1__c') return [SELECT Id FROM IspP1__c WITH USER_MODE ORDER BY Id].size();
        return [SELECT Id FROM IspP2__c WITH USER_MODE ORDER BY Id].size();
    }
}
