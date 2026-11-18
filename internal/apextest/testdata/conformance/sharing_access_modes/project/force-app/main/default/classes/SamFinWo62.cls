public without sharing class SamFinWo62 implements Finalizer {
  String tag;
  public SamFinWo62(String tag) { this.tag = tag; }
  public void execute(FinalizerContext ctx) { SamLog.put(tag, 'sys=' + [SELECT COUNT() FROM Sam_Sh__c WITH SYSTEM_MODE] + ' def=' + [SELECT COUNT() FROM Sam_Sh__c] + ' usr=' + [SELECT COUNT() FROM Sam_Sh__c WITH USER_MODE]); }
}
