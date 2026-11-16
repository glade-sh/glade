public class EbProbe63 {
  static String ex(Exception e){ String m=e.getMessage(); m=m==null?'null':m.replaceAll('\\s+',' '); return e.getTypeName()+'|'+m; }
  static String sr(Database.SaveResult r){ return 'ok='+r.isSuccess()+(r.isSuccess()?'':' '+r.getErrors()[0].getStatusCode()+'|'+r.getErrors()[0].getMessage()); }
  public static String run(){
    List<String> o=new List<String>();
    try{ o.add('EB1 '+sr(EventBus.publish(new Probe__e(Msg__c='a')))); }catch(Exception e){ o.add('EB1 '+ex(e)); }
    try{ o.add('EB2 '+sr(EventBus.publishWithAccessLevel(new Probe__e(Msg__c='a'),AccessLevel.SYSTEM_MODE))); }catch(Exception e){ o.add('EB2 '+ex(e)); }
    try{ o.add('EB3 '+sr(EventBus.publishWithAccessLevel(new Probe__e(Msg__c='a'),AccessLevel.USER_MODE))); }catch(Exception e){ o.add('EB3 '+ex(e)); }
    try{ o.add('EB4 '+sr(EventBus.publishWithAccessLevel(new Probe__e(Msg__c='a'),null))); }catch(Exception e){ o.add('EB4 '+ex(e)); }
    try{ List<Database.SaveResult> r=EventBus.publishWithAccessLevel(new List<Probe__e>{new Probe__e(Msg__c='a'),new Probe__e(Msg__c='b')},AccessLevel.USER_MODE); o.add('EB5 n='+r.size()+' '+sr(r[0])); }catch(Exception e){ o.add('EB5 '+ex(e)); }
    try{ List<Database.SaveResult> r=EventBus.publish(new List<Probe__e>{new Probe__e(Msg__c='a')}); o.add('EB6 n='+r.size()+' '+sr(r[0])); }catch(Exception e){ o.add('EB6 '+ex(e)); }
    try{ o.add('EB7 '+sr(EventBus.publishWithAccessLevel((SObject)new Probe__e(Msg__c='a'),AccessLevel.USER_MODE))); }catch(Exception e){ o.add('EB7 '+ex(e)); }
    try{ List<Database.SaveResult> r=EventBus.publishWithAccessLevel(new List<SObject>(),AccessLevel.USER_MODE); o.add('EB8 n='+r.size()); }catch(Exception e){ o.add('EB8 '+ex(e)); }
    return String.join(o,' ## ');
  }
}
