public with sharing class FamilyL07Oracle {
 @AuraEnabled(cacheable=true)
 public static Map<String,Object> echo(String marker, Integer ordinal) {
  if(marker=='__THROW__') throw new AuraHandledException('L07 controlled error');
  if(marker=='__NULL_RETURN__') return null;
  return new Map<String,Object>{'marker'=>marker,'ordinal'=>ordinal,
   'nested'=>new Map<String,Object>{'value'=>'owned'}, 'items'=>new List<Integer>{1,2}};
 }
 @AuraEnabled
 public static void changeOwned(Id recordId, String name) {
  Account a=[SELECT Id,Name FROM Account WHERE Id=:recordId LIMIT 1];
  if(!a.Name.startsWith('L07_ORACLE_') || !name.startsWith('L07_ORACLE_'))
   throw new AuraHandledException('L07 ownership boundary');
  a.Name=name; update a;
 }
}