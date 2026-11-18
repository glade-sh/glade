public without sharing class SamBWo68 implements Database.Batchable<Integer> {
  String tag;
  public SamBWo68(String tag) { this.tag = tag; }
  public Iterable<Integer> start(Database.BatchableContext bc) { return new List<Integer>{1}; }
  public void execute(Database.BatchableContext bc, List<Integer> scope) { SamLog.put(tag, 'sys=' + [SELECT COUNT() FROM Sam_Sh__c WITH SYSTEM_MODE] + ' def=' + [SELECT COUNT() FROM Sam_Sh__c] + ' usr=' + [SELECT COUNT() FROM Sam_Sh__c WITH USER_MODE]); }
  public void finish(Database.BatchableContext bc) {}
}
