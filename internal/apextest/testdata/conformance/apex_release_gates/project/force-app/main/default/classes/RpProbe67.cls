public class RpProbe67 {
  static String ex(Exception e){ String m=e.getMessage(); m=m==null?'null':m.replaceAll('id [a-zA-Z0-9]{15,18}','id ID').replaceAll('\\s+',' '); return e.getTypeName()+'|'+m; }
  public static String run(){
    List<String> o=new List<String>();
    RpMaster__c m1=new RpMaster__c(Name='m1'), m2=new RpMaster__c(Name='m2'); insert new List<RpMaster__c>{m1,m2};
    RpChild__c c=new RpChild__c(Name='c',Master__c=m1.Id); insert c;
    RpChildR__c cr=new RpChildR__c(Name='cr',Master__c=m1.Id); insert cr;
    try{ SObject x=new RpChild__c(); x.put('Master__c',m2.Id); x.put('Id',c.Id); update x; o.add('RP1 ok parent='+par(c.Id,m2.Id)); }catch(Exception e){ o.add('RP1 '+ex(e)); }
    try{ SObject x=new RpChild__c(); x.put('Id',c.Id); try{ x.put('Master__c',m2.Id); o.add('RP2 put ok'); }catch(Exception e){ o.add('RP2 put '+ex(e)); } update x; o.add('RP2 ok parent='+par(c.Id,m2.Id)); }catch(Exception e){ o.add('RP2 '+ex(e)); }
    try{ RpChild__c x=new RpChild__c(); x.Master__c=m2.Id; x.Id=c.Id; update x; o.add('RP3 ok parent='+par(c.Id,m2.Id)); }catch(Exception e){ o.add('RP3 '+ex(e)); }
    try{ RpChild__c x=new RpChild__c(); x.Id=c.Id; x.Master__c=m2.Id; update x; o.add('RP3b ok parent='+par(c.Id,m2.Id)); }catch(Exception e){ o.add('RP3b '+ex(e)); }
    try{ RpChild__c x=[SELECT Id,Master__c FROM RpChild__c WHERE Id=:c.Id]; x.Master__c=m2.Id; update x; o.add('RP4 ok parent='+par(c.Id,m2.Id)); }catch(Exception e){ o.add('RP4 '+ex(e)); }
    try{ SObject x=[SELECT Id,Master__c FROM RpChild__c WHERE Id=:c.Id]; x.put('Master__c',m2.Id); update x; o.add('RP4b ok parent='+par(c.Id,m2.Id)); }catch(Exception e){ o.add('RP4b '+ex(e)); }
    try{ RpChild__c x=new RpChild__c(Id=c.Id); x.Master__c=m2.Id; update x; o.add('RP5 ok parent='+par(c.Id,m2.Id)); }catch(Exception e){ o.add('RP5 '+ex(e)); }
    try{ RpChild__c x=new RpChild__c(Id=c.Id,Master__c=m2.Id); update x; o.add('RP5b ok parent='+par(c.Id,m2.Id)); }catch(Exception e){ o.add('RP5b '+ex(e)); }
    try{ RpChild__c x=new RpChild__c(Name='n',Master__c=m2.Id); insert x; o.add('RP6 ok'); }catch(Exception e){ o.add('RP6 '+ex(e)); }
    try{ SObject x=new RpChild__c(); x.put('Master__c',m2.Id); x.put('Id',c.Id); Database.SaveResult r=Database.update(x,false); o.add('RP7 success='+r.isSuccess()+(r.isSuccess()?'':' '+r.getErrors()[0].getStatusCode()+'|'+r.getErrors()[0].getMessage()+'|'+r.getErrors()[0].getFields())); }catch(Exception e){ o.add('RP7 '+ex(e)); }
    try{ SObject x=new RpChildR__c(); x.put('Master__c',m2.Id); x.put('Id',cr.Id); update x; o.add('RP8 ok parent='+(([SELECT Master__c FROM RpChildR__c WHERE Id=:cr.Id][0].Master__c==m2.Id)?'m2':'m1')); }catch(Exception e){ o.add('RP8 '+ex(e)); }
    try{ SObject x=new RpChild__c(); x.put('Master__c',[SELECT Master__c FROM RpChild__c WHERE Id=:c.Id][0].Master__c); x.put('Id',c.Id); update x; o.add('RP9 ok'); }catch(Exception e){ o.add('RP9 '+ex(e)); }
    try{ SObject x=new RpChild__c(); x.put('Master__c',null); x.put('Id',c.Id); update x; o.add('RP10 ok'); }catch(Exception e){ o.add('RP10 '+ex(e)); }
    try{ RpChild__c x=new RpChild__c(Id=c.Id); x.Master__c=m2.Id; upsert x; o.add('RP11 ok'); }catch(Exception e){ o.add('RP11 '+ex(e)); }
    try{ SObject x=new RpChild__c(); x.put('Master__c',m2.Id); x.put('Id',c.Id); Database.UpsertResult r=Database.upsert(x,false); o.add('RP12 success='+r.isSuccess()+(r.isSuccess()?'':' '+r.getErrors()[0].getStatusCode())); }catch(Exception e){ o.add('RP12 '+ex(e)); }
    try{ RpChild__c x=[SELECT Id,Name,Master__c FROM RpChild__c WHERE Id=:c.Id]; x.Name='q13'; update x; o.add('RP13 ok name='+[SELECT Name FROM RpChild__c WHERE Id=:c.Id][0].Name+' parent='+par(c.Id,m2.Id)); }catch(Exception e){ o.add('RP13 '+ex(e)); }
    try{ RpChild__c x=[SELECT Id,Name FROM RpChild__c WHERE Id=:c.Id]; x.Name='q14'; update x; o.add('RP14 ok name='+[SELECT Name FROM RpChild__c WHERE Id=:c.Id][0].Name); }catch(Exception e){ o.add('RP14 '+ex(e)); }
    try{ RpChild__c x=new RpChild__c(Id=c.Id,Name='q15'); update x; o.add('RP15 ok name='+[SELECT Name FROM RpChild__c WHERE Id=:c.Id][0].Name); }catch(Exception e){ o.add('RP15 '+ex(e)); }
    try{ RpChild__c x=[SELECT Id,Name,Master__c FROM RpChild__c WHERE Id=:c.Id]; x.Name='q16'; Database.SaveResult r=Database.update(x,false); o.add('RP16 success='+r.isSuccess()+(r.isSuccess()?'':' '+r.getErrors()[0].getStatusCode())); }catch(Exception e){ o.add('RP16 '+ex(e)); }
    try{ List<RpChild__c> xs=[SELECT Id,Name,Master__c FROM RpChild__c WHERE Id=:c.Id]; xs[0].Name='q17'; update xs; o.add('RP17 ok name='+[SELECT Name FROM RpChild__c WHERE Id=:c.Id][0].Name); }catch(Exception e){ o.add('RP17 '+ex(e)); }
    try{ RpChild__c x=[SELECT Id,Name,Master__c FROM RpChild__c WHERE Id=:c.Id]; x.Name='q18'; update as system x; o.add('RP18 ok name='+[SELECT Name FROM RpChild__c WHERE Id=:c.Id][0].Name); }catch(Exception e){ o.add('RP18 '+ex(e)); }
    try{ SObject x=new RpChild__c(); x.put('Master__c',[SELECT Master__c FROM RpChild__c WHERE Id=:c.Id][0].Master__c); x.put('Id',c.Id); update as system x; o.add('RP19 ok'); }catch(Exception e){ o.add('RP19 '+ex(e)); }
    try{ SObject x=new RpChild__c(); x.put('Master__c',m2.Id); x.put('Id',c.Id); update as system x; o.add('RP20 ok parent='+par(c.Id,m2.Id)); }catch(Exception e){ o.add('RP20 '+ex(e)); }
    return String.join(o,' ## ');
  }
  static String par(Id cid, Id m2id){ return [SELECT Master__c FROM RpChild__c WHERE Id=:cid][0].Master__c==m2id?'m2':'m1'; }
}
