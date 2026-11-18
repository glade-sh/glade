public class PgProbe66 {
  static String ex(Exception e){ String m=e.getMessage(); m=m==null?'null':m.replaceAll('\\s+',' '); return e.getTypeName()+'|'+m; }
  public static String run(){
    List<String> o=new List<String>();
    try{ o.add('PG1 '+System.purgeOldAsyncJobs(Date.today(),100)); }catch(Exception e){ o.add('PG1 '+ex(e)); }
    try{ o.add('PG1b '+System.purgeOldAsyncJobs(Date.today())); }catch(Exception e){ o.add('PG1b '+ex(e)); }
    try{ o.add('PG2 '+System.purgeOldAsyncJobs(Date.today(),0)); }catch(Exception e){ o.add('PG2 '+ex(e)); }
    try{ o.add('PG3 '+System.purgeOldAsyncJobs(Date.today(),-5)); }catch(Exception e){ o.add('PG3 '+ex(e)); }
    try{ o.add('PG4 '+System.purgeOldAsyncJobs(null,10)); }catch(Exception e){ o.add('PG4 '+ex(e)); }
    try{ o.add('PG4b '+System.purgeOldAsyncJobs(null)); }catch(Exception e){ o.add('PG4b '+ex(e)); }
    try{ o.add('PG5 '+System.purgeOldAsyncJobs(Date.today().addDays(-30),50)); }catch(Exception e){ o.add('PG5 '+ex(e)); }
    try{ o.add('PG6 '+System.purgeOldAsyncJobs(Date.today().addDays(5),null)); }catch(Exception e){ o.add('PG6 '+ex(e)); }
    try{ Integer b=Limits.getDmlStatements(); Integer b2=Limits.getDmlRows(); Integer r=System.purgeOldAsyncJobs(Date.today(),10); o.add('PG7 dml='+(Limits.getDmlStatements()-b)+' rows='+(Limits.getDmlRows()-b2)); }catch(Exception e){ o.add('PG7 '+ex(e)); }
    return String.join(o,' ## ');
  }
}
