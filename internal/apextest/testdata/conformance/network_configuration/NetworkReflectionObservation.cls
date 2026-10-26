public class NetworkReflectionObservation {
 public static String observe() {
  Map<String,String> rows = new Map<String,String>();
  rows.put('qualifiedType',String.valueOf(Type.forName('Schema.Network') != null));
  rows.put('unqualifiedType',String.valueOf(Type.forName('Network') != null));
  rows.put('explicitSchemaType',String.valueOf(Type.forName('Schema','Network') != null));
  rows.put('memberAvailable',String.valueOf(Schema.getGlobalDescribe().containsKey('NetworkMember')));
  if (Schema.getGlobalDescribe().containsKey('NetworkMember')) rows.put('memberCount',String.valueOf(Database.countQuery('SELECT COUNT() FROM NetworkMember')));
  return JSON.serialize(rows);
 }
}
