public with sharing class SamQFin62 implements Queueable {
  String kind; String tag;
  public SamQFin62(String kind, String tag) { this.kind = kind; this.tag = tag; }
  public void execute(QueueableContext ctx) { if (kind == 'W') System.attachFinalizer(new SamFinW62(tag)); if (kind == 'Wo') System.attachFinalizer(new SamFinWo62(tag)); if (kind == 'Inh') System.attachFinalizer(new SamFinInh62(tag)); if (kind == 'None') System.attachFinalizer(new SamFinNone62(tag)); }
}
