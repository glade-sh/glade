public class ProfileVisibilityProbe {
 public static String sample() {
  Map<String,Object> rows = new Map<String,Object>();
  Integer missingNames = 0;
  for (Profile item : [SELECT Id,Name FROM Profile]) {
   if (item.Name == null) missingNames++;
  }
  rows.put('missingProfileNames', missingNames);
  List<User> processUsers = [SELECT Id,ProfileId FROM User WHERE UserType = 'AutomatedProcess'];
  rows.put('hasProcessUser', !processUsers.isEmpty());
  Set<Id> processProfileIds = new Set<Id>();
  for (User item : processUsers) {
   if (item.ProfileId != null) processProfileIds.add(item.ProfileId);
  }
  List<Profile> processProfiles = [SELECT Id,Name FROM Profile WHERE Id IN :processProfileIds ORDER BY Name];
  rows.put('processProfiles', processProfiles.size());
  List<String> names = new List<String>();
  for (Profile item : processProfiles) names.add(item.Name);
  rows.put('processProfileNames', names);
  return JSON.serialize(rows);
 }
}
