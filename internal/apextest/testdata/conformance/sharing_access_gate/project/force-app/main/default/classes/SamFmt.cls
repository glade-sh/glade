public class SamFmt {
  static List<String> prefixes;
  public static String mask(String s) {
    if (s == null) return 'null';
    if (prefixes == null) {
      prefixes = new List<String>{'001','003','005','006','00Q','00e','01s','0PS','707','00G'};
      for (Schema.SObjectType t : new List<Schema.SObjectType>{Sam_Obj__c.SObjectType, Sam_RO__c.SObjectType, Sam_None__c.SObjectType, Sam_Sh__c.SObjectType, Sam_Log__c.SObjectType}) { prefixes.add(t.getDescribe().getKeyPrefix()); }
    }
    for (String p : prefixes) { s = s.replaceAll('\\b' + p + '[A-Za-z0-9]{12}([A-Za-z0-9]{3})?\\b', '[ID]'); }
    return s;
  }
  public static String ex(Exception e) {
    String s = 'EX ' + e.getTypeName() + ': ' + e.getMessage();
    if (e instanceof DmlException) {
      DmlException d = (DmlException) e;
      s += ' | n=' + d.getNumDml();
      if (d.getNumDml() > 0) { s += ' st=' + String.valueOf(d.getDmlType(0)) + ' f=' + String.valueOf(d.getDmlFieldNames(0)); }
    }
    if (e instanceof QueryException) { s += ' | inacc=' + String.valueOf(((QueryException) e).getInaccessibleFields()); }
    return mask(s);
  }
  static String errs(List<Database.Error> l) {
    String s = '';
    for (Database.Error er : l) { s += ' ' + String.valueOf(er.getStatusCode()) + ':' + er.getMessage() + ':' + String.valueOf(er.getFields()); }
    return s;
  }
  public static String sr(List<Database.SaveResult> r) {
    String s = '';
    for (Database.SaveResult x : r) { s += (x.isSuccess() ? 'ok' : 'FAIL') + errs(x.getErrors()) + ';'; }
    return mask(s);
  }
  public static String dr(List<Database.DeleteResult> r) {
    String s = '';
    for (Database.DeleteResult x : r) { s += (x.isSuccess() ? 'ok' : 'FAIL') + errs(x.getErrors()) + ';'; }
    return mask(s);
  }
  public static String ur(List<Database.UpsertResult> r) {
    String s = '';
    for (Database.UpsertResult x : r) { s += (x.isSuccess() ? 'ok' : 'FAIL') + (x.isCreated() ? ' created' : '') + errs(x.getErrors()) + ';'; }
    return mask(s);
  }
  public static String strip(SObjectAccessDecision d) {
    List<String> keys = new List<String>(d.getRemovedFields().keySet());
    keys.sort();
    String s = 'rm=';
    for (String k : keys) { List<String> f = new List<String>(d.getRemovedFields().get(k)); f.sort(); s += k + '=' + String.join(f, ',') + ';'; }
    s += ' recs=';
    for (SObject r : d.getRecords()) { List<String> f = new List<String>(r.getPopulatedFieldsAsMap().keySet()); f.sort(); s += String.join(f, ',') + ';'; }
    s += ' mod=' + String.valueOf(d.getModifiedIndexes());
    return mask(s);
  }
}
