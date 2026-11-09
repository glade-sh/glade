public with sharing class RunAsWithSharing { public static Integer countPriv() { return [SELECT COUNT() FROM RunAsPriv__c WHERE Name='AdminSeed']; } }
