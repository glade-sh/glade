public class LdProbe {
 public static String error(Exception fault) { return fault.getTypeName()+'|'+fault.getMessage(); }
 public static LoadRow__c one(String name) { return [SELECT Name,Body__c,Num__c,Int__c,Flag__c,Day__c,Stamp__c,Mail__c,Phone__c,Choice__c,Pct__c,Money__c,Required__c,Parent__c FROM LoadRow__c WHERE Name=:name LIMIT 1]; }
}
