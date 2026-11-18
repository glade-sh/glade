public with sharing class SamQUser67 implements Queueable {
  String tag; Id expected;
  public SamQUser67(String tag, Id expected) { this.tag = tag; this.expected = expected; }
  public void execute(QueueableContext ctx) { SamLog.put(tag, 'same=' + (UserInfo.getUserId() == expected)); }
}
