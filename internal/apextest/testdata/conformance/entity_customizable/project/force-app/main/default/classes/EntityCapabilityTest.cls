@IsTest private class EntityCapabilityTest {
 @IsTest static void customizableFlags() {
  Map<String,Object> result=new Map<String,Object>();
  for(EntityDefinition row:[SELECT QualifiedApiName,IsCustomizable,IsTriggerable FROM EntityDefinition WHERE QualifiedApiName IN ('Account','Contact','User','PricebookEntry','ApexClass','UserLogin','EntityDefinition') ORDER BY QualifiedApiName]) {
   result.put(row.QualifiedApiName,new List<Boolean>{row.IsCustomizable,row.IsTriggerable});
  }
  System.assert(false,'OBSERVED|customizableFlags|'+JSON.serialize(result));
 }
 @IsTest static void customizableFilter() {
  List<String> names=new List<String>();
  for(EntityDefinition row:[SELECT QualifiedApiName FROM EntityDefinition WHERE QualifiedApiName IN ('Account','Contact','User','PricebookEntry','ApexClass','UserLogin','EntityDefinition') AND IsCustomizable=true AND IsTriggerable=true ORDER BY QualifiedApiName]) {
   names.add(row.QualifiedApiName);
  }
  System.assert(false,'OBSERVED|customizableFilter|'+JSON.serialize(new Map<String,Object>{'names'=>names}));
 }
}
