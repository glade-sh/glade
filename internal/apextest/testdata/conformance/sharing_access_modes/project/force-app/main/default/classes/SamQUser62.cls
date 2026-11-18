public with sharing class SamQUser62 implements Queueable {
  String tag; Id expected;
  public SamQUser62(String tag, Id expected) { this.tag = tag; this.expected = expected; }
  public void execute(QueueableContext ctx) { SamLog.put(tag, 'same=' + (UserInfo.getUserId() == expected)); }
}
