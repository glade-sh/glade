public class PeProbe {
 public static List<String> names() {
  List<String> out=new List<String>();
  for (Account row:[SELECT Name FROM Account WHERE Name LIKE 'PE-%' ORDER BY Name]) out.add(row.Name);
  return out;
 }
 public static String error(Exception fault) { return fault.getTypeName()+'|'+fault.getMessage(); }
}
