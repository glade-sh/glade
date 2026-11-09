public class MeadowProbe {
 public static String observe(String shape) {
  if (shape == 'custom-column') return [SELECT Column__r.QualifiedApiName FROM MeadowField__mdt WHERE DeveloperName = 'CustomState'].Column__r.QualifiedApiName;
  if (shape == 'standard-column') return [SELECT Column__r.QualifiedApiName FROM MeadowField__mdt WHERE DeveloperName = 'StandardRating'].Column__r.QualifiedApiName;
  if (shape == 'entity') return [SELECT Entity__r.QualifiedApiName FROM MeadowField__mdt WHERE DeveloperName = 'CustomState'].Entity__r.QualifiedApiName;
  Map<String,List<String>> groups = new Map<String,List<String>>();
  for (MeadowGroup__mdt groupRow : [SELECT DeveloperName, Descriptor__r.Column__r.QualifiedApiName, Descriptor__r.Entity__r.QualifiedApiName, (SELECT Value__c FROM Choices__r) FROM MeadowGroup__mdt WHERE Descriptor__r.Entity__r.QualifiedApiName = 'MeadowEntry__c']) {
   if (groupRow.Descriptor__r.Column__r.QualifiedApiName == 'State__c') {
    groups.put(groupRow.DeveloperName, new List<String>());
    for (MeadowChoice__mdt choice : groupRow.Choices__r) groups.get(groupRow.DeveloperName).add(choice.Value__c);
   }
  }
  try { return String.valueOf(groups.get('Ready').contains('Ready')); }
  catch (Exception e) { return e.getTypeName(); }
 }
}
