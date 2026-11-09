public class TWHelper {
  public static String describe(Exception e) {
    String s = e.getTypeName() + '|' + e.getMessage() + '|cause=' + (e.getCause() == null ? 'null' : e.getCause().getTypeName() + ':' + e.getCause().getMessage()) + '|stack=' + e.getStackTraceString();
    if (e instanceof DmlException) {
      DmlException d = (DmlException) e;
      s += '|n=' + d.getNumDml() + '|dmlMsg=' + d.getDmlMessage(0) + '|dmlType=' + d.getDmlType(0) + '|fields=' + d.getDmlFieldNames(0) + '|idx=' + d.getDmlIndex(0) + '|code=' + d.getDmlStatusCode(0);
    }
    return s;
  }
  public static String errs(Database.SaveResult sr) {
    String s = 'ok=' + sr.isSuccess() + ' id=' + (sr.getId() != null);
    for (Database.Error er : sr.getErrors()) s += ' [code=' + er.getStatusCode() + ' msg=' + er.getMessage() + ' fields=' + er.getFields() + ']';
    return s;
  }
  public static String ins(String name) {
    try { insert new Account(Name = name); return 'noexc'; } catch (Exception e) { return describe(e); }
  }
  public static String dbFalse(String name) {
    try { return errs(Database.insert(new Account(Name = name), false)); } catch (Exception e) { return 'EXC ' + describe(e); }
  }
  public static String dbTrue(String name) {
    try { return errs(Database.insert(new Account(Name = name), true)); } catch (Exception e) { return 'EXC ' + describe(e); }
  }
  public static String mixedFalse(String name) {
    try {
      List<Database.SaveResult> rs = Database.insert(new List<Account>{new Account(Name = 'good1'), new Account(Name = name), new Account(Name = 'good2')}, false);
      String s = '';
      for (Database.SaveResult r : rs) s += '{' + errs(r) + '}';
      return s + ' count=' + [SELECT COUNT() FROM Account WHERE Name LIKE 'good%'];
    } catch (Exception e) { return 'EXC ' + describe(e); }
  }
  public static String mixedList(String name) {
    try { insert new List<Account>{new Account(Name = 'good1'), new Account(Name = name)}; return 'noexc'; }
    catch (Exception e) { return describe(e) + ' count=' + [SELECT COUNT() FROM Account WHERE Name LIKE 'good%']; }
  }
  public static String cell(String kind, String name) {
    if (kind == 'ins') return ins(name);
    if (kind == 'dbF') return dbFalse(name);
    if (kind == 'dbT') return dbTrue(name);
    if (kind == 'mixF') return mixedFalse(name);
    return mixedList(name);
  }
  public static List<String> keys() { return new List<String>{'b-api-nomsg','b-api-false','b-api-eq','b-api-ne','b-api-null','b-api-notnull','b-api-fail','b-api-inst','b-cls-sys','b-cls-api','b-cls-deep','b-cls-throw','a-api','a-cls-deep','b-assert','b-eq','b-nomsg','b-ne','b-assertapi','b-adderr','b-throw','b-swallow','a-assert','a-eq','a-nomsg','a-adderr'}; }
}