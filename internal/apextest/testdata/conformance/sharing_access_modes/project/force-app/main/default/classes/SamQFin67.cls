public with sharing class SamQFin67 implements Queueable {
  String kind; String tag;
  public SamQFin67(String kind, String tag) { this.kind = kind; this.tag = tag; }
  public void execute(QueueableContext ctx) { if (kind == 'W') System.attachFinalizer(new SamFinW67(tag)); if (kind == 'Wo') System.attachFinalizer(new SamFinWo67(tag)); if (kind == 'Inh') System.attachFinalizer(new SamFinInh67(tag)); if (kind == 'None') System.attachFinalizer(new SamFinNone67(tag)); }
}
