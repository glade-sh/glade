public class NamespaceLookupProbe {
 public static String sample() {
  Map<String,Object> rows = new Map<String,Object>();
  ApexClass ownClass = [SELECT Name,NamespacePrefix FROM ApexClass WHERE Name = 'NamespaceLookupProbe' LIMIT 1];
  rows.put('className', ownClass.Name);
  rows.put('namespaceNull', ownClass.NamespacePrefix == null);
  String qualifiedName = ownClass.Name;
  if (ownClass.NamespacePrefix != null) qualifiedName = ownClass.NamespacePrefix + '.' + qualifiedName;
  try {
   Type resolved = Type.forName(qualifiedName);
   rows.put('resolved', resolved != null);
   if (resolved != null) rows.put('constructed', resolved.newInstance() != null);
  } catch (Exception problem) {
   rows.put('reflectionError', problem.getTypeName());
  }
  Id currentProfileId = UserInfo.getProfileId();
  Profile currentProfile = [SELECT Name FROM Profile WHERE Id = :currentProfileId LIMIT 1];
  rows.put('profileNamePresent', currentProfile.Name != null);
  return JSON.serialize(rows);
 }
}
