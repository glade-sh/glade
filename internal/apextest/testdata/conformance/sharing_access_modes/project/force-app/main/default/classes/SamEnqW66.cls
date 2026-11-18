public with sharing class SamEnqW66 {
  public static void q(String kind, String tag) { if (kind == 'W') System.enqueueJob(new SamQW66(tag)); if (kind == 'Wo') System.enqueueJob(new SamQWo66(tag)); if (kind == 'Inh') System.enqueueJob(new SamQInh66(tag)); if (kind == 'None') System.enqueueJob(new SamQNone66(tag)); }
  public static void b(String kind, String tag) { if (kind == 'W') Database.executeBatch(new SamBW66(tag), 1); if (kind == 'Wo') Database.executeBatch(new SamBWo66(tag), 1); if (kind == 'Inh') Database.executeBatch(new SamBInh66(tag), 1); if (kind == 'None') Database.executeBatch(new SamBNone66(tag), 1); }
  public static void f(String kind, String tag) { if (kind == 'W') SamFW66.run(tag); if (kind == 'Wo') SamFWo66.run(tag); if (kind == 'Inh') SamFInh66.run(tag); if (kind == 'None') SamFNone66.run(tag); }
  public static void fin(String kind, String tag) { System.enqueueJob(new SamQFin66(kind, tag)); }
  public static void chain(String tag) { System.enqueueJob(new SamQChain66(tag)); }
}
