public class CxLog61 {
  public static String go() {
    String s = 'noexc';
    try {
      Database.SaveResult r = Database.insert(new Cx__c(Name = 'n', Msg__c = 'x'), AccessLevel.USER_MODE);
      s = 'SR ok=' + r.isSuccess();
      for (Database.Error e : r.getErrors()) { s += ' ' + String.valueOf(e.getStatusCode()) + ':' + e.getMessage() + ':' + String.valueOf(e.getFields()); }
    } catch (Exception e) {
      s = 'EX ' + e.getTypeName() + ': ' + e.getMessage();
      if (e instanceof DmlException) { DmlException x = (DmlException) e; if (x.getNumDml() > 0) { s += ' | st=' + String.valueOf(x.getDmlType(0)) + ' f=' + String.valueOf(x.getDmlFieldNames(0)); } }
    }
    return s;
  }
}