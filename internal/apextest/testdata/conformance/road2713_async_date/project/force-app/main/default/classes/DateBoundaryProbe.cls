public class DateBoundaryProbe {
 public static String observe(String s) {
  try { Date d=Date.parse(s); return 'OK|'+String.valueOf(d); }
  catch(Exception e) { return e.getTypeName()+'|'+e.getMessage(); }
 }
}
