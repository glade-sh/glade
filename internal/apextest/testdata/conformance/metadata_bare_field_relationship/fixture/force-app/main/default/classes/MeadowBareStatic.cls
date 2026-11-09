public class MeadowBareStatic {
 private static Map<String,List<String>> groups = build();
 private static List<String> ready = groups.get('ReadyBare');
 private static Map<String,List<String>> build() {
  Map<String,List<String>> m = new Map<String,List<String>>();
  String objectName = MeadowEntry__c.SObjectType.getDescribe().getName();
  String fieldName = MeadowEntry__c.State__c.getDescribe().getName();
  for (MeadowGroup__mdt g : [SELECT DeveloperName, Descriptor__r.Column__r.QualifiedApiName, Descriptor__r.Entity__r.QualifiedApiName, (SELECT Value__c FROM Choices__r) FROM MeadowGroup__mdt WHERE Descriptor__r.Entity__r.QualifiedApiName = :objectName]) {
   if (g.Descriptor__r.Column__r.QualifiedApiName == fieldName) {
    m.put(g.DeveloperName, new List<String>());
    for (MeadowChoice__mdt c : g.Choices__r) m.get(g.DeveloperName).add(c.Value__c);
   }
  }
  return m;
 }
 public static Boolean has(String s) { return ready.contains(s); }
}
