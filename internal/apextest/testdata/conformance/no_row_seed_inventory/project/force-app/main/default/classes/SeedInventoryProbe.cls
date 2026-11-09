public class SeedInventoryProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> result = new Map<String,Object>();
  result.put('usersWithRole', [SELECT COUNT() FROM User WHERE UserRoleId != null]);
  result.put('currentHasRole', [SELECT UserRoleId FROM User WHERE Id = :UserInfo.getUserId()].UserRoleId != null);
  result.put('accountMasterRecordType', [SELECT COUNT() FROM RecordType WHERE SObjectType = 'Account' AND DeveloperName = 'Master']);
  result.put('missingRecordType', [SELECT COUNT() FROM RecordType WHERE SObjectType = 'Account' AND DeveloperName = 'Synthetic_Missing_92711']);
  List<String> roleNames = new List<String>();
  for (UserRole r : [SELECT Name, DeveloperName, ParentRole.DeveloperName FROM UserRole]) roleNames.add(r.DeveloperName + '|' + r.Name + '|' + (r.ParentRole == null ? '' : r.ParentRole.DeveloperName));
  roleNames.sort();
  result.put('roleNames', roleNames);
  result.put('roles', roleNames.size());
  result.put('queues', [SELECT COUNT() FROM Group WHERE Type = 'Queue']);
  result.put('standardProfile', [SELECT COUNT() FROM Profile WHERE Name = 'Standard User']);
  result.put('missingProfile', [SELECT COUNT() FROM Profile WHERE Name = 'Synthetic Missing Profile 92711']);
  result.put('missingQueue', [SELECT COUNT() FROM Group WHERE Type = 'Queue' AND DeveloperName = 'Synthetic_Missing_Queue_92711']);
  result.put('standardPricebook', [SELECT COUNT() FROM Pricebook2 WHERE IsStandard = true]);
  result.put('defaultBusinessHours', [SELECT COUNT() FROM BusinessHours WHERE IsDefault = true]);
  try { User u = [SELECT Id FROM User WHERE UserRoleId != null LIMIT 1]; result.put('roleUserSingleton','found'); }
  catch (QueryException e) { result.put('roleUserSingleton', e.getMessage()); }
  return result;
 }
}
