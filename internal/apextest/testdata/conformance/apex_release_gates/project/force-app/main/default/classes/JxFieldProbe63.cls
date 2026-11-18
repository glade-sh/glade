public class JxFieldProbe63 {
  public class FException extends Exception { public Integer k=3; public String s='z'; }
  public virtual class BaseException extends Exception { public Integer b=1; }
  public class KidException extends BaseException { public Integer c=2; }
  public static String run(){
    List<String> o=new List<String>();
    try{ o.add('JX12 '+JSON.serialize(new FException('m'))); }catch(Exception e){ o.add('JX12 '+e.getTypeName()+'|'+e.getMessage()); }
    try{ o.add('JX13 '+JSON.serialize(new KidException())); }catch(Exception e){ o.add('JX13 '+e.getTypeName()+'|'+e.getMessage()); }
    try{ o.add('JX14 '+JSON.serialize(new List<Object>{1,new FException('m')})); }catch(Exception e){ o.add('JX14 '+e.getTypeName()+'|'+e.getMessage()); }
    try{ o.add('JX15 '+JSON.serialize(new Map<String,Object>{'a'=>new Map<String,Object>{'b'=>new NullPointerException()}})); }catch(Exception e){ o.add('JX15 '+e.getTypeName()+'|'+e.getMessage()); }
    try{ o.add('JX16 '+JSON.serialize(new FException('m'),true)); }catch(Exception e){ o.add('JX16 '+e.getTypeName()+'|'+e.getMessage()); }
    try{ o.add('JX17 '+JSON.serialize(new Wrap())); }catch(Exception e){ o.add('JX17 '+e.getTypeName()+'|'+e.getMessage()); }
    try{ o.add('JX18 '+JSON.serialize(new CalloutException('c'))); }catch(Exception e){ o.add('JX18 '+e.getTypeName()+'|'+e.getMessage()); }
    return String.join(o,' ## ');
  }
  public class Wrap { public FException ex=new FException('w'); public Integer n=1; }
}
