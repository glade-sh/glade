public class NumericCaptureBridge {
 public static void putObject(SObject r,Schema.SObjectField f,Object x){r.put(f,x);}
 public static void putInteger(SObject r,Schema.SObjectField f,Integer x){r.put(f,x);}
 public static Object getToken(SObject r,Schema.SObjectField f){return r.get(f);}
}