public class ZeroObjectSort68 {
 static Double p() { return Double.valueOf('0.0'); }
 static Double n() { return Double.valueOf('-0.0'); }
 public static String orders() { List<Object> a=new List<Object>{p(),n()},b=new List<Object>{n(),p()}; a.sort(); b.sort(); return a+'|'+b; }
 public static String duplicates() { List<Object> a=new List<Object>{p(),n(),p(),n()},b=new List<Object>{p(),p()},c=new List<Object>{n(),n()}; a.sort(); b.sort(); c.sort(); return a+'|'+b+'|'+c; }
 public static String boundaries() { List<Object> a=new List<Object>{p(),Double.valueOf('1.0'),n(),Double.valueOf('-1.0'),null}; List<Object> b=new List<Object>{Decimal.valueOf('0.0'),Decimal.valueOf('-0.0')}; a.sort(); b.sort(); return a+'|'+b; }
}
