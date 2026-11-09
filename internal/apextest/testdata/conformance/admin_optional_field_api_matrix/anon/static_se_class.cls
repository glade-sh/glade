public class SeProbe { public static Integer run(){ return [SELECT Id, Optional__c FROM CapObj__c WITH SECURITY_ENFORCED].size(); } }
