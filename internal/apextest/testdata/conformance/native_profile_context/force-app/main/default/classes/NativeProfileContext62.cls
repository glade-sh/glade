public class NativeProfileContext62 {
 public static Map<String,Object> current() {
  Profile currentProfile = [SELECT Name FROM Profile WHERE Id = :UserInfo.getProfileId()];
  Map<String,Object> row = new Map<String,Object>{'profile'=>currentProfile.Name};
  row.put('restricted', save('restricted'));
  row.put('standardOnly', save('standard-only'));
  row.put('ordinary', save('ordinary'));
  row.put('unassigned', [SELECT count() FROM PermissionSetAssignment WHERE AssigneeId = :UserInfo.getUserId() AND PermissionSet.Name = 'Native_Context_Access']);
  Account source = new Account(Name='Owned Context');
  insert source;
  Account selected = [SELECT Id,Description FROM Account WHERE Id = :source.Id];
  row.put('selectedNullPopulated', selected.getPopulatedFieldsAsMap().containsKey('Description'));
  return row;
 }
 public static String save(String name) {
  Database.SaveResult result = Database.insert(new ContextRecord__c(Name=name), false);
  return result.isSuccess() ? 'ok' : String.valueOf(result.getErrors()[0].getStatusCode());
 }
}
