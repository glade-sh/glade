public class RpModeProbe67 {
  public static String ex(Exception e){ String m=e.getMessage(); m=m==null?'null':m.replaceAll('id [a-zA-Z0-9]{15,18}','id ID').replaceAll('\\s+',' '); return e.getTypeName()+'|'+m; }
  static String nm(Id cid){ return [SELECT Name FROM RpChild__c WHERE Id=:cid][0].Name; }
  static String sr(Database.SaveResult r){ return r.isSuccess()?'ok':''+r.getErrors()[0].getStatusCode(); }
  public without sharing class W { public String go(Id cid){ try{ RpChild__c x=[SELECT Id,Name,Master__c FROM RpChild__c WHERE Id=:cid]; x.Name='w'; update x; return 'ok'; }catch(Exception e){ return RpModeProbe67.ex(e);} } }
  public with sharing class S { public String go(Id cid){ try{ RpChild__c x=[SELECT Id,Name,Master__c FROM RpChild__c WHERE Id=:cid]; x.Name='s'; update x; return 'ok'; }catch(Exception e){ return RpModeProbe67.ex(e);} } }
  public inherited sharing class I { public String go(Id cid){ try{ RpChild__c x=[SELECT Id,Name,Master__c FROM RpChild__c WHERE Id=:cid]; x.Name='i'; update x; return 'ok'; }catch(Exception e){ return RpModeProbe67.ex(e);} } }
  public static String run(){
    List<String> o=new List<String>();
    RpMaster__c m1=new RpMaster__c(Name='m1'), m2=new RpMaster__c(Name='m2'); insert new List<RpMaster__c>{m1,m2};
    RpChild__c c=new RpChild__c(Name='c',Master__c=m1.Id); insert c;
    RpChild__c c2=new RpChild__c(Name='c2',Master__c=m1.Id); insert c2;
    RpChildR__c cr=new RpChildR__c(Name='cr',Master__c=m1.Id); insert cr;
    try{ RpChild__c x=new RpChild__c(Name='n',Master__c=m1.Id); insert x; x.Name='n2'; update x; o.add('RM1 ok name='+nm(x.Id)); }catch(Exception e){ o.add('RM1 '+ex(e)); }
    try{ RpChild__c x=new RpChild__c(Name='n',Master__c=m1.Id); insert x; x.Name='n2'; update as system x; o.add('RM2 ok name='+nm(x.Id)); }catch(Exception e){ o.add('RM2 '+ex(e)); }
    try{ RpChild__c x=new RpChild__c(Name='n',Master__c=m1.Id); insert x; x.Name='n2'; update as user x; o.add('RM3 ok name='+nm(x.Id)); }catch(Exception e){ o.add('RM3 '+ex(e)); }
    try{ RpChild__c x=[SELECT Id,Name,Master__c FROM RpChild__c WHERE Id=:c.Id]; x.Name='q4'; update as user x; o.add('RM4 ok name='+nm(c.Id)); }catch(Exception e){ o.add('RM4 '+ex(e)); }
    try{ SObject x=new RpChild__c(); x.put('Master__c',m2.Id); x.put('Id',c.Id); update as user x; o.add('RM5 ok parent='+([SELECT Master__c FROM RpChild__c WHERE Id=:c.Id][0].Master__c==m2.Id?'m2':'m1')); }catch(Exception e){ o.add('RM5 '+ex(e)); }
    try{ RpChild__c x=[SELECT Id,Name,Master__c FROM RpChild__c WHERE Id=:c.Id]; x.Name='q6'; Database.SaveResult r=Database.update(x,false,AccessLevel.USER_MODE); o.add('RM6 '+sr(r)); }catch(Exception e){ o.add('RM6 '+ex(e)); }
    try{ RpChildR__c x=[SELECT Id,Name,Master__c FROM RpChildR__c WHERE Id=:cr.Id]; x.Name='q7'; update x; o.add('RM7 ok name='+[SELECT Name FROM RpChildR__c WHERE Id=:cr.Id][0].Name); }catch(Exception e){ o.add('RM7 '+ex(e)); }
    try{ RpChild__c x=[SELECT Id,Name,Master__c FROM RpChild__c WHERE Id=:c.Id]; x.Name='q8'; upsert x; o.add('RM8 ok name='+nm(c.Id)); }catch(Exception e){ o.add('RM8 '+ex(e)); }
    try{ RpChild__c x=[SELECT Id,Name,Master__r.Name FROM RpChild__c WHERE Id=:c.Id]; x.Name='q9'; update x; o.add('RM9 ok name='+nm(c.Id)); }catch(Exception e){ o.add('RM9 '+ex(e)); }
    try{ RpChild__c x=[SELECT Id,Name,Master__c FROM RpChild__c WHERE Id=:c.Id]; x.Name='q10'; RpChild__c y=[SELECT Id,Name FROM RpChild__c WHERE Id=:c2.Id]; y.Name='y10'; List<Database.SaveResult> r=Database.update(new List<RpChild__c>{x,y},false); o.add('RM10 s0='+sr(r[0])+' s1='+sr(r[1])+' names='+nm(c.Id)+','+nm(c2.Id)); }catch(Exception e){ o.add('RM10 '+ex(e)); }
    o.add('RM11 '+new W().go(c.Id));
    o.add('RM12 '+new S().go(c.Id));
    o.add('RM13 '+new I().go(c.Id));
    try{ RpModeHold.child=c.Id; RpModeHold.res='not run'; insert new RpMaster2__c(Name='t'); o.add('RM14 '+RpModeHold.res); }catch(Exception e){ o.add('RM14 '+ex(e)); }
    try{ RpChild__c x=new RpChild__c(Name='n',Master__c=m1.Id); insert x; x.Name='n2'; Database.SaveResult r=Database.update(x,false); o.add('RM15 '+sr(r)+' name='+nm(x.Id)); }catch(Exception e){ o.add('RM15 '+ex(e)); }
    try{ RpChild__c x=[SELECT Id,Name,Master__c FROM RpChild__c WHERE Id=:c.Id]; x.Name='q16'; update new List<RpChild__c>{x}; o.add('RM16 ok'); }catch(Exception e){ o.add('RM16 '+ex(e)); }
    return String.join(o,' ## ');
  }
}
