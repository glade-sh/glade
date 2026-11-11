public class OrderV67 {
 static List<String> names(List<Profile> rows) {List<String> out=new List<String>();for(Profile p:rows)out.add(p.Name);return out;}
 public static Object observe() {
  Map<String,Object> out=new Map<String,Object>();
  List<Profile> allRows=[SELECT Name FROM Profile ORDER BY Name];
  List<Profile> firstRows=new List<Profile>();for(Integer i=0;i<allRows.size() && i<5;i++)firstRows.add(allRows[i]);out.put('allCount',allRows.size());out.put('allFirst',names(firstRows));
  out.put('limit1',names([SELECT Name FROM Profile ORDER BY Name LIMIT 1]));
  out.put('limit2',names([SELECT Name FROM Profile ORDER BY Name LIMIT 2]));
  out.put('limit3',names([SELECT Name FROM Profile ORDER BY Name LIMIT 3]));
  out.put('limit5',names([SELECT Name FROM Profile ORDER BY Name LIMIT 5]));
  out.put('offset1',names([SELECT Name FROM Profile ORDER BY Name LIMIT 1 OFFSET 1]));
  out.put('descending',names([SELECT Name FROM Profile ORDER BY Name DESC LIMIT 1]));
  out.put('nonnull',names([SELECT Name FROM Profile WHERE Name != null ORDER BY Name LIMIT 1]));
  out.put('license',names([SELECT Name,UserLicense.Name FROM Profile ORDER BY Name LIMIT 1]));
  return out;
 }
}
