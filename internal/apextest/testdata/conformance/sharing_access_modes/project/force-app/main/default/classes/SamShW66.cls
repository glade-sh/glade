public with sharing class SamShW66 {
  public static String cnt() { return 'sys=' + [SELECT COUNT() FROM Sam_Sh__c WITH SYSTEM_MODE] + ' def=' + [SELECT COUNT() FROM Sam_Sh__c] + ' usr=' + [SELECT COUNT() FROM Sam_Sh__c WITH USER_MODE]; }
  public static String upd(Id id, String how) {
    Sam_Sh__c r = new Sam_Sh__c(Id = id, Name = 'u');
    List<Sam_Sh__c> l = new List<Sam_Sh__c>{r};
    try {
      if (how == 'def') update r;
      if (how == 'usr') update as user r;
      if (how == 'sys') update as system r;
      if (how == 'dbdf') return SamFmt.sr(Database.update(l, false));
      if (how == 'dbuf') return SamFmt.sr(Database.update(l, false, AccessLevel.USER_MODE));
      if (how == 'dbut') return SamFmt.sr(Database.update(l, true, AccessLevel.USER_MODE));
      if (how == 'dbsf') return SamFmt.sr(Database.update(l, false, AccessLevel.SYSTEM_MODE));
      return 'ok';
    } catch (Exception e) { return SamFmt.ex(e); }
  }
  public static String del(Id id, String how) {
    Sam_Sh__c r = new Sam_Sh__c(Id = id);
    try {
      if (how == 'def') delete r;
      if (how == 'usr') delete as user r;
      if (how == 'sys') delete as system r;
      if (how == 'dbuf') return SamFmt.dr(Database.delete(new List<Id>{id}, false, AccessLevel.USER_MODE));
      if (how == 'dbsf') return SamFmt.dr(Database.delete(new List<Id>{id}, false, AccessLevel.SYSTEM_MODE));
      return 'ok';
    } catch (Exception e) { return SamFmt.ex(e); }
  }
  public static String delNone(Id id, String how) {
    Sam_None__c r = new Sam_None__c(Id = id);
    try {
      if (how == 'def') delete r;
      if (how == 'usr') delete as user r;
      if (how == 'sys') delete as system r;
      return 'ok';
    } catch (Exception e) { return SamFmt.ex(e); }
  }
  public static String q6(String obj, String shape, Id id) {
    try {
      if (obj == 'none') {
        if (shape == 'cnt') return 'n=' + [SELECT COUNT() FROM Sam_None__c];
        if (shape == 'cntw') return 'n=' + [SELECT COUNT() FROM Sam_None__c WHERE Name = 'x'];
        if (shape == 'cntid') return 'n=' + [SELECT COUNT() FROM Sam_None__c WHERE Id = :id];
        if (shape == 'lim') return 'n=' + [SELECT Id FROM Sam_None__c LIMIT 1].size();
        if (shape == 'ord') return 'n=' + [SELECT Id FROM Sam_None__c ORDER BY Name].size();
        if (shape == 'dyn') return 'n=' + Database.countQuery('SELECT COUNT() FROM Sam_None__c WHERE Name = \'x\'');
        if (shape == 'cntu') return 'n=' + [SELECT COUNT() FROM Sam_None__c WITH USER_MODE];
      } else {
        if (shape == 'cnt') return 'n=' + [SELECT COUNT() FROM Account];
        if (shape == 'cntw') return 'n=' + [SELECT COUNT() FROM Account WHERE Name = 'x'];
        if (shape == 'cntid') return 'n=' + [SELECT COUNT() FROM Account WHERE Id = :id];
        if (shape == 'lim') return 'n=' + [SELECT Id FROM Account LIMIT 1].size();
        if (shape == 'ord') return 'n=' + [SELECT Id FROM Account ORDER BY Name].size();
        if (shape == 'dyn') return 'n=' + Database.countQuery('SELECT COUNT() FROM Account WHERE Name = \'x\'');
        if (shape == 'cntu') return 'n=' + [SELECT COUNT() FROM Account WITH USER_MODE];
      }
      return '?';
    } catch (Exception e) { return SamFmt.ex(e); }
  }
  public static String pb() { return 'sys=' + [SELECT COUNT() FROM Pricebook2 WITH SYSTEM_MODE] + ' def=' + [SELECT COUNT() FROM Pricebook2]; }
}
