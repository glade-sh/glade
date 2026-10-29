public class RuntimeSchemaFactoryObject {
 public class DescribeSObjectResult {}
 public static Map<String,Object> observe() {
  Type shortLookup=Type.forName('DescribeSObjectResult');
  Type qualified=Schema.DescribeSObjectResult.class;
  Type nested=Type.forName('RuntimeSchemaFactoryObject.DescribeSObjectResult');
  return new Map<String,Object>{'shortLookup'=>shortLookup==qualified,'qualifiedLookup'=>Type.forName('Schema.DescribeSObjectResult')==qualified,'nestedFound'=>nested!=null,'nestedDistinct'=>nested!=qualified,'literalDistinct'=>DescribeSObjectResult.class!=qualified,'nullDistinct'=>shortLookup!=null};
 }
}
