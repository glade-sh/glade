public class ProfileProbeCase {
 public static String run() {
  String p = [SELECT Name FROM Profile WHERE Id = :UserInfo.getProfileId()].Name;
  List<Account> accts = new List<Account>{new Account(Name=p), new Account(Name='unmatched-probe')}; insert accts;
  List<Contact> cs = new List<Contact>{new Contact(LastName='ProfileProbe',AccountId=accts[0].Id),new Contact(LastName='ProfileProbe',AccountId=accts[1].Id),new Contact(LastName='ProfileProbe')}; insert cs;
  Map<Id,Contact> got = new Map<Id,Contact>([SELECT Description FROM Contact WHERE Id IN :cs]);
  return got.get(cs[0].Id).Description+'|'+got.get(cs[1].Id).Description+'|'+got.get(cs[2].Id).Description;
 }
}