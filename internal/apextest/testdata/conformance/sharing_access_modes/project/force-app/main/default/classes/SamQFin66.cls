public with sharing class SamQFin66 implements Queueable {
  String kind; String tag;
  public SamQFin66(String kind, String tag) { this.kind = kind; this.tag = tag; }
  public void execute(QueueableContext ctx) { if (kind == 'W') System.attachFinalizer(new SamFinW66(tag)); if (kind == 'Wo') System.attachFinalizer(new SamFinWo66(tag)); if (kind == 'Inh') System.attachFinalizer(new SamFinInh66(tag)); if (kind == 'None') System.attachFinalizer(new SamFinNone66(tag)); }
}
