public without sharing class SamLog {
  public static void put(String tag, String note) { insert as system new Sam_Log__c(Name = tag.left(80), Note__c = tag + ' ' + note); }
  public static String all() {
    String s = '';
    for (Sam_Log__c l : [SELECT Note__c FROM Sam_Log__c WITH SYSTEM_MODE ORDER BY Name]) { s += l.Note__c + ' ;; '; }
    return s;
  }
}
