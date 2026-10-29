public class RuntimeSchemaFactoryField {
 public class DescribeFieldResult {}
 public static Map<String,Object> observe() {
  Type shortLookup=Type.forName('DescribeFieldResult');
  Type qualified=Schema.DescribeFieldResult.class;
  Type nested=Type.forName('RuntimeSchemaFactoryField.DescribeFieldResult');
  return new Map<String,Object>{'shortLookup'=>shortLookup==qualified,'qualifiedLookup'=>Type.forName('Schema.DescribeFieldResult')==qualified,'nestedFound'=>nested!=null,'nestedDistinct'=>nested!=qualified,'literalDistinct'=>DescribeFieldResult.class!=qualified,'nullDistinct'=>shortLookup!=null};
 }
}
