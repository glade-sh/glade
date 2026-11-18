public class HdCtl { public static Integer depth = 0; public static String mode = 'plain'; public static String log = ''; public static String lim = '';
  public static void bounce(SObject o) {
    depth++; Integer d = depth;
    SObject next = o instanceof HdA__c ? (SObject)[SELECT Id FROM HdB__c LIMIT 1] : (SObject)[SELECT Id FROM HdA__c LIMIT 1];
    next.put('Name', 'n' + d);
    if (mode == 'catch') { try { update next; } catch (DmlException e) { log += d + 'C,'; } finally { log += d + 'F,'; } }
    else if (mode == 'swallow') { try { update next; } catch (DmlException e) { log += d + 'C,'; lim = 'dml=' + Limits.getDmlStatements() + ',rows=' + Limits.getDmlRows() + ',q=' + Limits.getQueries() + ',qr=' + Limits.getQueryRows(); } }
    else if (mode == 'partial') { Database.SaveResult r = Database.update(next, false); if (!r.isSuccess()) { log += d + 'P,'; } }
    else { update next; }
  }
  public static void reset(String m) { depth = 0; mode = m; log = ''; lim = ''; }
}