public without sharing class PrivateSetupReader { public static Integer countRows() { return [SELECT COUNT() FROM PrivateProbe__c WHERE Name='Seed']; } }
