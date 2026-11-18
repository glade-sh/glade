public class ZA67 {
 static Double p() { return Double.valueOf('0.0'); }
 static Double n() { return Double.valueOf('-0.0'); }
 static Double o() { return Double.valueOf('1.0'); }
 public static String d01() { List<Double> a=new List<Double>{p(),o()},b=new List<Double>{n(),o()}; System.assertEquals(a,b); return 'ok'; }
 public static String d02() { List<Double> a=new List<Double>{p(),o()},b=new List<Double>{n(),o()}; System.assertNotEquals(a,b); return 'ok'; }
 public static String d03() { List<Double> a=new List<Double>{p(),o()},b=new List<Double>{n(),o()}; Assert.areEqual(a,b); return 'ok'; }
 public static String d04() { List<Double> a=new List<Double>{p(),o()},b=new List<Double>{n(),o()}; Assert.areNotEqual(a,b); return 'ok'; }
 public static String d05() { Double a=p(),b=n(); System.assertEquals(a,b); return 'ok'; }
 public static String d06() { Set<Double> a=new Set<Double>{p()},b=new Set<Double>{n()}; System.assertEquals(a,b); return 'ok'; }
 public static String d07() { Map<String,Double> a=new Map<String,Double>{'k'=>p()},b=new Map<String,Double>{'k'=>n()}; System.assertEquals(a,b); return 'ok'; }
}
