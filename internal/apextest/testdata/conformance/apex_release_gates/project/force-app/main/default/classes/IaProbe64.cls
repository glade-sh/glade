public class IaProbe64 {
  static String ex(Exception e){ String m=e.getMessage(); m=m==null?'null':m.replaceAll('\\s+',' '); return e.getTypeName()+'|'+m; }
  public static String run(){
    List<String> o=new List<String>();
    try{ Invocable.Action a=Invocable.Action.createStandardAction('chatterPost'); o.add('IA1 std ver='+a.getVersion()+' type='+a.getType()+' name='+a.getName()); }catch(Exception e){ o.add('IA1 '+ex(e)); }
    try{ Invocable.Action a=Invocable.Action.createCustomAction('apex','Foo'); o.add('IA2 ver='+a.getVersion()+' type='+a.getType()+' name='+a.getName()); }catch(Exception e){ o.add('IA2 '+ex(e)); }
    try{ Invocable.Action a=Invocable.Action.createCustomAction('apex','ns','Foo','1'); o.add('IA2d ver='+a.getVersion()+' name='+a.getName()); }catch(Exception e){ o.add('IA2d '+ex(e)); }
    try{ Invocable.Action a=Invocable.Action.createCustomAction('flow','NoSuchFlow'); List<Invocable.Action.Result> r=a.invoke(); o.add('IA3 n='+r.size()); }catch(Exception e){ o.add('IA3 '+ex(e)); }
    try{ Invocable.Action a=Invocable.Action.createStandardAction('chatterPost'); List<Invocable.Action.Result> r=a.invoke(); o.add('IA4 n='+(r==null?-1:r.size())); }catch(Exception e){ o.add('IA4 '+ex(e)); }
    try{ Invocable.Action a=Invocable.Action.createStandardAction('chatterPost'); List<Invocable.Action.Result> r=a.getDescribe()==null?null:null; o.add('IA5 describe='+String.valueOf(a.getDescribe()!=null)); }catch(Exception e){ o.add('IA5 '+ex(e)); }
    try{ Invocable.Action a=Invocable.Action.createStandardAction('nosuchaction'); o.add('IA6 ver='+a.getVersion()); }catch(Exception e){ o.add('IA6 '+ex(e)); }
    return String.join(o,' ## ');
  }
}
