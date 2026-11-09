public class AccessProbe62 {
 private static String describe(Database.SaveResult r) { if (r.isSuccess()) return 'success'; Database.Error e=r.getErrors()[0]; return 'failure:'+e.getStatusCode()+':'+e.getMessage(); }
 public static String run() { String s='';
  try { s+='scalar='+describe(EventBus.publish(new ProbeSignal__e())); } catch(Exception e) { s+='scalar=exception:'+e.getTypeName()+':'+e.getMessage(); }
  try { s+=',second='+describe(EventBus.publish(new AlertSignal__e())); } catch(Exception e) { s+=',second=exception:'+e.getTypeName()+':'+e.getMessage(); }
  try { s+=',list='+describe(EventBus.publish(new List<ProbeSignal__e>{new ProbeSignal__e()})[0]); } catch(Exception e) { s+=',list=exception:'+e.getTypeName()+':'+e.getMessage(); }
  try { s+=',generic='+describe(EventBus.publish(new List<SObject>{new ProbeSignal__e()})[0]); } catch(Exception e) { s+=',generic=exception:'+e.getTypeName()+':'+e.getMessage(); }
  return s;
 }
}
