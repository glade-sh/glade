public class ProfileProbeCase {
 public static String run() {
  String p = [SELECT Name FROM Profile WHERE Id = :UserInfo.getProfileId()].Name;
  List<Contact> cs = new List<Contact>{new Contact(LastName='ProfileProbe', Title=p), new Contact(LastName='ProfileProbe', Title='definitely different'), new Contact(LastName='ProfileProbe')};
  insert cs;
  List<String> out = new List<String>();
  for (Contact c : [SELECT Description FROM Contact WHERE Id IN :cs ORDER BY Title ASC NULLS LAST]) out.add(String.valueOf(c.Description));
  return String.join(out, '|');
 }
}
