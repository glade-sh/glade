public without sharing class RunAsWithoutSharing { public static Integer countPriv() { return [SELECT COUNT() FROM RunAsPriv__c WHERE Name='AdminSeed']; } }
