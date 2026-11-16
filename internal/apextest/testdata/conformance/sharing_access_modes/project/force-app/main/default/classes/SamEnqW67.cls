public with sharing class SamEnqW67 {
  public static void q(String kind, String tag) { if (kind == 'W') System.enqueueJob(new SamQW67(tag)); if (kind == 'Wo') System.enqueueJob(new SamQWo67(tag)); if (kind == 'Inh') System.enqueueJob(new SamQInh67(tag)); if (kind == 'None') System.enqueueJob(new SamQNone67(tag)); }
  public static void b(String kind, String tag) { if (kind == 'W') Database.executeBatch(new SamBW67(tag), 1); if (kind == 'Wo') Database.executeBatch(new SamBWo67(tag), 1); if (kind == 'Inh') Database.executeBatch(new SamBInh67(tag), 1); if (kind == 'None') Database.executeBatch(new SamBNone67(tag), 1); }
  public static void f(String kind, String tag) { if (kind == 'W') SamFW67.run(tag); if (kind == 'Wo') SamFWo67.run(tag); if (kind == 'Inh') SamFInh67.run(tag); if (kind == 'None') SamFNone67.run(tag); }
  public static void fin(String kind, String tag) { System.enqueueJob(new SamQFin67(kind, tag)); }
  public static void chain(String tag) { System.enqueueJob(new SamQChain67(tag)); }
}
