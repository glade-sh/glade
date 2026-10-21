public with sharing class MxQ24 {
    public static PrbLedger__c fetch(Id rid) { return Database.query('SELECT Id,Name,Amount__c,Count__c FROM PrbLedger__c WHERE Id = :rid'); }
}