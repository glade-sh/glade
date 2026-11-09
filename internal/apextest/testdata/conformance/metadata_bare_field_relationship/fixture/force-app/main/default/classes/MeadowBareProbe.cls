public class MeadowBareProbe {
 public static String observe(String shape) {
  if (shape == 'bare-column') return [SELECT Column__r.QualifiedApiName FROM MeadowField__mdt WHERE DeveloperName = 'BareState'].Column__r.QualifiedApiName;
  if (shape == 'bare-standard-column') return [SELECT Column__r.QualifiedApiName FROM MeadowField__mdt WHERE DeveloperName = 'BareRating'].Column__r.QualifiedApiName;
  if (shape == 'bare-entity') return [SELECT Entity__r.QualifiedApiName FROM MeadowField__mdt WHERE DeveloperName = 'BareState'].Entity__r.QualifiedApiName;
  if (shape == 'bare-column-durable') return String.valueOf([SELECT Column__r.DurableId FROM MeadowField__mdt WHERE DeveloperName = 'BareState'].Column__r.DurableId);
  if (shape == 'bare-groups') {
   Map<String,List<String>> groups = new Map<String,List<String>>();
   for (MeadowGroup__mdt g : [SELECT DeveloperName, Descriptor__r.Column__r.QualifiedApiName, Descriptor__r.Entity__r.QualifiedApiName, (SELECT Value__c FROM Choices__r) FROM MeadowGroup__mdt WHERE Descriptor__r.Entity__r.QualifiedApiName = 'MeadowEntry__c']) {
    if (g.Descriptor__r.Column__r.QualifiedApiName == 'State__c') {
     groups.put(g.DeveloperName, new List<String>());
     for (MeadowChoice__mdt c : g.Choices__r) groups.get(g.DeveloperName).add(c.Value__c);
    }
   }
   return JSON.serialize(groups);
  }
  return MeadowBareStatic.has('Ready') ? 'true' : 'false';
 }
}
