public class ProfileProbeCase {
 public static String run() {
  String p = [SELECT Name FROM Profile WHERE Id = :UserInfo.getProfileId()].Name;
  List<Contact> cs = new List<Contact>{new Contact(LastName='ProfileProbe', Title=p),new Contact(LastName='ProfileProbe', Title=p.toLowerCase()),new Contact(LastName='ProfileProbe', Title=' '+p+' ')}; insert cs;
  Map<Id,Contact> got = new Map<Id,Contact>([SELECT Description FROM Contact WHERE Id IN :cs]);
  return got.get(cs[0].Id).Description+'|'+got.get(cs[1].Id).Description+'|'+got.get(cs[2].Id).Description;
 }
}
