public class SeedSelectorProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> result = new Map<String,Object>();
  result.put('activeNonSystem', ![SELECT Id FROM User WHERE IsActive = true AND (NOT Profile.Name LIKE 'System%') LIMIT 1].isEmpty());
  result.put('activeUser', [SELECT COUNT() FROM User WHERE Id = :UserInfo.getUserId() AND IsActive = true]);
  result.put('standardProfile', [SELECT COUNT() FROM Profile WHERE Name = 'Standard User']);
  result.put('missingProfile', [SELECT COUNT() FROM Profile WHERE Name = 'Synthetic Missing Profile 92711']);
  result.put('queue', [SELECT COUNT() FROM Group WHERE Type = 'Queue' AND DeveloperName = 'Synthetic_Missing_Queue_92711']);
  try { Profile p = [SELECT Id FROM Profile WHERE Name = 'Synthetic Missing Profile 92711' LIMIT 1]; result.put('missingError','none'); }
  catch (QueryException e) { result.put('missingError',e.getMessage()); }
  return result;
 }
}
