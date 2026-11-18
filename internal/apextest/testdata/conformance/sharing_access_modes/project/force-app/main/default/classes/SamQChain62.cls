public without sharing class SamQChain62 implements Queueable {
  String tag;
  public SamQChain62(String tag) { this.tag = tag; }
  public void execute(QueueableContext ctx) {
    SamLog.put(tag + '_outer', 'sys=' + [SELECT COUNT() FROM Sam_Sh__c WITH SYSTEM_MODE] + ' def=' + [SELECT COUNT() FROM Sam_Sh__c] + ' usr=' + [SELECT COUNT() FROM Sam_Sh__c WITH USER_MODE]);
    try { System.enqueueJob(new SamQInh62(tag + '_inner')); } catch (Exception e) { SamLog.put(tag + '_enqueue', SamFmt.ex(e)); }
  }
}
