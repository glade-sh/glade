public class EhProbe64 {
  static String ex(Exception e){ String m=e.getMessage(); m=m==null?'null':m.replaceAll('\\s+',' '); return e.getTypeName()+'|'+m; }
  static String keys(Messaging.SingleEmailMessage m){ Map<String,String> h=m.getCustomHeaders(); if(h==null) return 'null'; List<String> k=new List<String>(h.keySet()); k.sort(); return String.join(k,',')+'='+h.size(); }
  static String vals(Messaging.SingleEmailMessage m){ Map<String,String> h=m.getCustomHeaders(); if(h==null) return 'null'; List<String> k=new List<String>(h.keySet()); k.sort(); List<String> r=new List<String>(); for(String x:k) r.add(x+':'+h.get(x)); return String.join(r,',');}
  public static String run(){
    List<String> o=new List<String>();
    Messaging.SingleEmailMessage m=new Messaging.SingleEmailMessage();
    try{ o.add('EH1 '+keys(m)); }catch(Exception e){ o.add('EH1 '+ex(e)); }
    try{ m=new Messaging.SingleEmailMessage(); m.setCustomHeaders(new Map<String,String>{'Probe'=>'yes'}); o.add('EH2 '+vals(m)); }catch(Exception e){ o.add('EH2 '+ex(e)); }
    try{ m=new Messaging.SingleEmailMessage(); m.setCustomHeaders(new Map<String,String>{'X-SFDC-X-Probe'=>'yes'}); o.add('EH3 '+vals(m)); }catch(Exception e){ o.add('EH3 '+ex(e)); }
    for(String n:new List<String>{'bad name','','x-sfdc-x-low','X-SFDC-X-','X-Sfdc-X-Mix','Reply-To','Subject','Content-Type'}){
      try{ m=new Messaging.SingleEmailMessage(); m.setCustomHeaders(new Map<String,String>{n=>'v'}); o.add('EH4 ['+n+'] '+vals(m)); }catch(Exception e){ o.add('EH4 ['+n+'] '+ex(e)); }
    }
    try{ m=new Messaging.SingleEmailMessage(); Map<String,String> h=new Map<String,String>(); h.put(null,'v'); m.setCustomHeaders(h); o.add('EH4n '+vals(m)); }catch(Exception e){ o.add('EH4n '+ex(e)); }
    try{ m=new Messaging.SingleEmailMessage(); m.setCustomHeaders(new Map<String,String>{'K'=>null}); o.add('EH4v '+vals(m)); }catch(Exception e){ o.add('EH4v '+ex(e)); }
    try{ m=new Messaging.SingleEmailMessage(); m.setCustomHeaders(null); o.add('EH4z '+keys(m)); }catch(Exception e){ o.add('EH4z '+ex(e)); }
    try{ m=new Messaging.SingleEmailMessage(); m.setCustomHeaders(new Map<String,String>{'K'=>'a\r\nb'}); o.add('EH5 '+vals(m).replace('\r','R').replace('\n','N')); }catch(Exception e){ o.add('EH5 '+ex(e)); }
    try{ m=new Messaging.SingleEmailMessage(); m.setCustomHeaders(new Map<String,String>{'Probe'=>'yes'}); m.setToAddresses(new List<String>{'a@example.invalid'}); m.setSubject('s'); m.setPlainTextBody('b'); Integer b=Limits.getEmailInvocations(); List<Messaging.SendEmailResult> r=Messaging.sendEmail(new List<Messaging.SingleEmailMessage>{m}); o.add('EH6 ok '+r[0].isSuccess()+' inv='+(Limits.getEmailInvocations()-b)+' '+vals(m)); }catch(Exception e){ o.add('EH6 '+ex(e)); }
    return String.join(o,' ## ');
  }
}
