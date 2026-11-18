public class AsyncProbe {
 public static void log(String step) {
  String name='AO-'+step.substring(0,Math.min(60,step.length()));
  insert new Account(Name=name, Description=step+'@'+String.valueOf(System.now()));
 }
 public static List<String> read() {
  List<String> out=new List<String>();
  for (Account row:[SELECT Name,Description FROM Account WHERE Name LIKE 'AO-%' ORDER BY CreatedDate,Id]) out.add(row.Description);
  return out;
 }
 @future public static void futureFromFinish(String token) { log('future-body-'+token); }
}
