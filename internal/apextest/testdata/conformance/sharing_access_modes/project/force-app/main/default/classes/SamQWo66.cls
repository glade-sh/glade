public without sharing class SamQWo66 implements Queueable {
  String tag;
  public SamQWo66(String tag) { this.tag = tag; }
  public void execute(QueueableContext ctx) { SamLog.put(tag, 'sys=' + [SELECT COUNT() FROM Sam_Sh__c WITH SYSTEM_MODE] + ' def=' + [SELECT COUNT() FROM Sam_Sh__c] + ' usr=' + [SELECT COUNT() FROM Sam_Sh__c WITH USER_MODE]); }
}
