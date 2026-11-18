public class PjProbe67 {
  static String ex(Exception e){ String m=e.getMessage(); m=m==null?'null':m.replaceAll('id [a-zA-Z0-9]{15,18}','id ID').replaceAll('[a-zA-Z0-9]{15,18}','ID').replaceAll('\\s+',' '); return e.getTypeName()+'|'+m; }
  public class J implements Schedulable { public void execute(SchedulableContext c){} }
  static String st(Id j){ return [SELECT State FROM CronTrigger WHERE Id=:j][0].State; }
  public static String run(){
    List<String> o=new List<String>();
    Id j=null; Integer b=0;
    try{ j=System.schedule('PJ-1','0 0 0 1 1 ? 2099',new J()); o.add('PJ0 state='+st(j)); }catch(Exception e){ o.add('PJ0 '+ex(e)); }
    try{ b=Limits.getDmlStatements(); System.pauseJobById(j); o.add('PJ1 state='+st(j)+' dml='+(Limits.getDmlStatements()-b)); }catch(Exception e){ o.add('PJ1 '+ex(e)); }
    try{ System.pauseJobById(j); o.add('PJ5 again state='+st(j)); }catch(Exception e){ o.add('PJ5 '+ex(e)); }
    try{ b=Limits.getDmlStatements(); System.resumeJobById(j); o.add('PJ2 state='+st(j)+' dml='+(Limits.getDmlStatements()-b)); }catch(Exception e){ o.add('PJ2 '+ex(e)); }
    try{ System.resumeJobById(j); o.add('PJ2b resume again state='+st(j)); }catch(Exception e){ o.add('PJ2b '+ex(e)); }
    try{ b=Limits.getDmlStatements(); System.pauseJobByName('PJ-1'); o.add('PJ3 state='+st(j)+' dml='+(Limits.getDmlStatements()-b)); }catch(Exception e){ o.add('PJ3 '+ex(e)); }
    try{ System.resumeJobByName('PJ-1'); o.add('PJ3b state='+st(j)); }catch(Exception e){ o.add('PJ3b '+ex(e)); }
    try{ System.pauseJobById('08e000000000000AAA'); o.add('PJ4 ok'); }catch(Exception e){ o.add('PJ4 '+ex(e)); }
    try{ System.pauseJobByName('none'); o.add('PJ4b ok'); }catch(Exception e){ o.add('PJ4b '+ex(e)); }
    try{ System.resumeJobById('08e000000000000AAA'); o.add('PJ4c ok'); }catch(Exception e){ o.add('PJ4c '+ex(e)); }
    try{ System.pauseJobById(null); o.add('PJ4d ok'); }catch(Exception e){ o.add('PJ4d '+ex(e)); }
    try{ System.resumeJobById('08e000000000000AAA'); o.add('PJ4c2 ok'); }catch(Exception e){ o.add('PJ4c2 '+ex(e)); }
    try{ System.resumeJobByName('none'); o.add('PJ4e ok'); }catch(Exception e){ o.add('PJ4e '+ex(e)); }
    try{ System.pauseJobByName(''); o.add('PJ4f ok'); }catch(Exception e){ o.add('PJ4f '+ex(e)); }
    try{ System.pauseJobByName(null); o.add('PJ4g ok'); }catch(Exception e){ o.add('PJ4g '+ex(e)); }
    try{ System.pauseJobById(''); o.add('PJ4h ok'); }catch(Exception e){ o.add('PJ4h '+ex(e)); }
    try{ b=Limits.getDmlStatements(); System.abortJob(j); o.add('PJ7 abort dml='+(Limits.getDmlStatements()-b)); }catch(Exception e){ o.add('PJ7 '+ex(e)); }
    return String.join(o,' ## ');
  }
}
