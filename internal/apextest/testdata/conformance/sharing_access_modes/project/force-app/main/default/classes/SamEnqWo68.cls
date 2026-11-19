public without sharing class SamEnqWo68 {
  public static void q(String kind, String tag) { if (kind == 'W') System.enqueueJob(new SamQW68(tag)); if (kind == 'Wo') System.enqueueJob(new SamQWo68(tag)); if (kind == 'Inh') System.enqueueJob(new SamQInh68(tag)); if (kind == 'None') System.enqueueJob(new SamQNone68(tag)); }
  public static void b(String kind, String tag) { if (kind == 'W') Database.executeBatch(new SamBW68(tag), 1); if (kind == 'Wo') Database.executeBatch(new SamBWo68(tag), 1); if (kind == 'Inh') Database.executeBatch(new SamBInh68(tag), 1); if (kind == 'None') Database.executeBatch(new SamBNone68(tag), 1); }
  public static void f(String kind, String tag) { if (kind == 'W') SamFW68.run(tag); if (kind == 'Wo') SamFWo68.run(tag); if (kind == 'Inh') SamFInh68.run(tag); if (kind == 'None') SamFNone68.run(tag); }
  public static void fin(String kind, String tag) { System.enqueueJob(new SamQFin68(kind, tag)); }
  public static void chain(String tag) { System.enqueueJob(new SamQChain68(tag)); }
}
