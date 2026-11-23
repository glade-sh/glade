public class ProfileProbeCase {
 public static String run() {
  String p = [SELECT Name FROM Profile WHERE Id = :UserInfo.getProfileId()].Name;
  Contact c = new Contact(LastName='ProfileProbe', Title=' '+p+' '); insert c;
  c = [SELECT Description,Title,Department FROM Contact WHERE Id=:c.Id];
  return c.Description+'|storedSpace='+String.valueOf(c.Title.startsWith(' '))+'|flowSpace='+String.valueOf(c.Department.startsWith(' '));
 }
}
