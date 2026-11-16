public class JxProbe62 {
  public class MyException extends Exception {}
  static String ex(Exception e){ String m=e.getMessage(); m=m==null?'null':m.replaceAll('id [a-zA-Z0-9]{15,18}','id ID').replaceAll('\\s+',' '); return e.getTypeName()+'|'+m; }
  static String san(String s){ return s==null?'null':s.replaceAll('"__lineNumber":[0-9]+','"__lineNumber":N').replaceAll('"__stackTrace":"[^"]*"','"__stackTrace":"S"'); }
  public static String run(){
    List<String> o=new List<String>();
    try{ o.add('JX1 '+san(JSON.serialize(new MyException('x')))); }catch(Exception e){ o.add('JX1 '+ex(e)); }
    try{ o.add('JX2 '+san(JSON.serialize(new DmlException('y')))); }catch(Exception e){ o.add('JX2 '+ex(e)); }
    try{ o.add('JX3 '+san(JSON.serialize(new Map<String,Object>{'k'=>new MyException('z')}))); }catch(Exception e){ o.add('JX3 '+ex(e)); }
    try{ try{ Integer.valueOf('x'); }catch(Exception e){ o.add('JX4 '+san(JSON.serialize(e))); } }catch(Exception e){ o.add('JX4 '+ex(e)); }
    try{ o.add('JX5 '+san(JSON.serialize(new List<Object>{new MyException('a'),new DmlException('b')}))); }catch(Exception e){ o.add('JX5 '+ex(e)); }
    try{ o.add('JX6 '+san(JSON.serializePretty(new MyException('p')))); }catch(Exception e){ o.add('JX6 '+ex(e)); }
    try{ o.add('JX7 '+san(JSON.serialize(new Map<String,String>{'a'=>'b'}))); }catch(Exception e){ o.add('JX7 '+ex(e)); }
    try{ o.add('JX9 '+san(JSON.serialize(new List<MyException>{new MyException('l')}))); }catch(Exception e){ o.add('JX9 '+ex(e)); }
    try{ o.add('JX10 '+san(JSON.serialize(new NullPointerException()))); }catch(Exception e){ o.add('JX10 '+ex(e)); }
    try{ Object r=JSON.deserializeUntyped('{"message":"x"}'); o.add('JX11 '+String.valueOf(r)); MyException m=(MyException)JSON.deserialize('{"message":"x"}',MyException.class); o.add('JX11b '+m.getMessage()); }catch(Exception e){ o.add('JX11 '+ex(e)); }
    return String.join(o,' ## ');
  }
}
