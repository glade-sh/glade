public without sharing class CedarDeleteMembers {
    @InvocableMethod
    public static void run(List<Cedar_Member__c> members) {
        delete members;
    }
}
