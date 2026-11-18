public with sharing class SamQFin68 implements Queueable {
  String kind; String tag;
  public SamQFin68(String kind, String tag) { this.kind = kind; this.tag = tag; }
  public void execute(QueueableContext ctx) { if (kind == 'W') System.attachFinalizer(new SamFinW68(tag)); if (kind == 'Wo') System.attachFinalizer(new SamFinWo68(tag)); if (kind == 'Inh') System.attachFinalizer(new SamFinInh68(tag)); if (kind == 'None') System.attachFinalizer(new SamFinNone68(tag)); }
}
