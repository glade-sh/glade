public with sharing class SamQUser66 implements Queueable {
  String tag; Id expected;
  public SamQUser66(String tag, Id expected) { this.tag = tag; this.expected = expected; }
  public void execute(QueueableContext ctx) { SamLog.put(tag, 'same=' + (UserInfo.getUserId() == expected)); }
}
