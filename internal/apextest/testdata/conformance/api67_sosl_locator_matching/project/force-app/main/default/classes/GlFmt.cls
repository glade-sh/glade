public class GlFmt {
  public static String ex(Exception e) {
    String s = 'EX ' + e.getTypeName() + ': ' + e.getMessage();
    try { if (e instanceof QueryException) { s += ' | inacc=' + String.valueOf(((QueryException)e).getInaccessibleFields()); } } catch (Exception x) {}
    try { if (e instanceof DmlException) { DmlException d=(DmlException)e; s += ' | f=' + String.valueOf(d.getDmlFieldNames(0)) + ' st=' + String.valueOf(d.getDmlType(0)) + ' n=' + d.getNumDml(); } } catch (Exception x) {}
    return s;
  }
  public static String sh(Exception e) {
    String s = e.getTypeName();
    try { if (e instanceof DmlException) { s += ':' + String.valueOf(((DmlException)e).getDmlType(0)); } } catch (Exception x) {}
    return s;
  }
  public static String sr(List<Database.SaveResult> r) {
    String s=''; for (Database.SaveResult x : r) { s += x.isSuccess() ? 'ok' : 'FAIL'; for (Database.Error er : x.getErrors()) s += ' ' + er.getStatusCode() + ':' + er.getMessage() + ':' + String.valueOf(er.getFields()); } return s;
  }
  public static String dr(List<Database.DeleteResult> r) {
    String s=''; for (Database.DeleteResult x : r) { s += x.isSuccess() ? 'ok' : 'FAIL'; for (Database.Error er : x.getErrors()) s += ' ' + er.getStatusCode() + ':' + er.getMessage() + ':' + String.valueOf(er.getFields()); } return s;
  }
  public static String ur(List<Database.UpsertResult> r) {
    String s=''; for (Database.UpsertResult x : r) { s += x.isSuccess() ? 'ok' : 'FAIL'; for (Database.Error er : x.getErrors()) s += ' ' + er.getStatusCode() + ':' + er.getMessage() + ':' + String.valueOf(er.getFields()); } return s;
  }
}