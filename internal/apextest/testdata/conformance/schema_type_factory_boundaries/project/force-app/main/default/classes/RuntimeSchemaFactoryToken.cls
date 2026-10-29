public class RuntimeSchemaFactoryToken {
 public class SObjectType {}
 public static Map<String,Object> observe() {
  Type shortLookup=Type.forName('SObjectType');
  Type qualified=Schema.SObjectType.class;
  Type nested=Type.forName('RuntimeSchemaFactoryToken.SObjectType');
  return new Map<String,Object>{'shortLookup'=>shortLookup==qualified,'qualifiedLookup'=>Type.forName('Schema.SObjectType')==qualified,'nestedFound'=>nested!=null,'nestedDistinct'=>nested!=qualified,'literalDistinct'=>SObjectType.class!=qualified,'nullDistinct'=>shortLookup!=null};
 }
}
