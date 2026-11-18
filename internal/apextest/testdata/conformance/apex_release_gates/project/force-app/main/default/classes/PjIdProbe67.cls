public class PjIdProbe67 {
  static String ex(Exception e){ return e.getTypeName()+'|'+e.getMessage(); }
  public static String run(){
    List<String> o=new List<String>();
    try{ System.pauseJobById('08e000000000001'); o.add('PI1 ok'); }catch(Exception e){ o.add('PI1 '+ex(e)); }
    try{ System.resumeJobById('08e000000000001'); o.add('PI2 ok'); }catch(Exception e){ o.add('PI2 '+ex(e)); }
    try{ System.pauseJobById('abc'); o.add('PI3 ok'); }catch(Exception e){ o.add('PI3 '+ex(e)); }
    try{ System.resumeJobById('abc'); o.add('PI4 ok'); }catch(Exception e){ o.add('PI4 '+ex(e)); }
    return String.join(o,' ## ');
  }
}
