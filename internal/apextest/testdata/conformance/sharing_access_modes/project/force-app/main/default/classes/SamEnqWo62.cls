public without sharing class SamEnqWo62 {
  public static void q(String kind, String tag) { if (kind == 'W') System.enqueueJob(new SamQW62(tag)); if (kind == 'Wo') System.enqueueJob(new SamQWo62(tag)); if (kind == 'Inh') System.enqueueJob(new SamQInh62(tag)); if (kind == 'None') System.enqueueJob(new SamQNone62(tag)); }
  public static void b(String kind, String tag) { if (kind == 'W') Database.executeBatch(new SamBW62(tag), 1); if (kind == 'Wo') Database.executeBatch(new SamBWo62(tag), 1); if (kind == 'Inh') Database.executeBatch(new SamBInh62(tag), 1); if (kind == 'None') Database.executeBatch(new SamBNone62(tag), 1); }
  public static void f(String kind, String tag) { if (kind == 'W') SamFW62.run(tag); if (kind == 'Wo') SamFWo62.run(tag); if (kind == 'Inh') SamFInh62.run(tag); if (kind == 'None') SamFNone62.run(tag); }
  public static void fin(String kind, String tag) { System.enqueueJob(new SamQFin62(kind, tag)); }
  public static void chain(String tag) { System.enqueueJob(new SamQChain62(tag)); }
}
