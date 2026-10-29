public class RuntimeSchemaFactoryMember {
 public class SObjectField {}
 public static Map<String,Object> observe() {
  Type shortLookup=Type.forName('SObjectField');
  Type qualified=Schema.SObjectField.class;
  Type nested=Type.forName('RuntimeSchemaFactoryMember.SObjectField');
  return new Map<String,Object>{'shortLookup'=>shortLookup==qualified,'qualifiedLookup'=>Type.forName('Schema.SObjectField')==qualified,'nestedFound'=>nested!=null,'nestedDistinct'=>nested!=qualified,'literalDistinct'=>SObjectField.class!=qualified,'nullDistinct'=>shortLookup!=null};
 }
}
