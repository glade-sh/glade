public class CritMFCase {
 public static String mf() {
  List<String> out = new List<String>();
  for (String t : new List<String>{'1. Alpha Item', '22.  beta item ', 'Missing', 'x. ', null}) {
   Contact c = new Contact(LastName='CritMF', Title=t);
   Database.SaveResult r = Database.insert(c, false);
   if (!r.isSuccess()) { out.add('ERR:' + String.valueOf(r.getErrors()[0].getStatusCode()) + ':' + r.getErrors()[0].getMessage()); continue; }
   out.add(String.valueOf([SELECT Description FROM Contact WHERE Id = :c.Id].Description));
  }
  return String.join(out, '|');
 }
}
