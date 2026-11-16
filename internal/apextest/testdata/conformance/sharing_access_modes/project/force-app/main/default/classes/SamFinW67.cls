public with sharing class SamFinW67 implements Finalizer {
  String tag;
  public SamFinW67(String tag) { this.tag = tag; }
  public void execute(FinalizerContext ctx) { SamLog.put(tag, 'sys=' + [SELECT COUNT() FROM Sam_Sh__c WITH SYSTEM_MODE] + ' def=' + [SELECT COUNT() FROM Sam_Sh__c] + ' usr=' + [SELECT COUNT() FROM Sam_Sh__c WITH USER_MODE]); }
}
