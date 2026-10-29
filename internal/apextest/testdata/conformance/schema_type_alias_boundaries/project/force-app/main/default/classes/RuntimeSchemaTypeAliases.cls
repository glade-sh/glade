public class RuntimeSchemaTypeAliases {
 public class DescribeFieldResult {}
 private static Map<String,Object> pair(Type shortName,Type qualified) {
  Map<Type,String> keyed=new Map<Type,String>{shortName=>'retained'};
  return new Map<String,Object>{'equal'=>shortName==qualified,'methodEqual'=>shortName.equals(qualified),'setSize'=>new Set<Type>{shortName,qualified}.size(),'mapLookup'=>keyed.get(qualified),'shortName'=>shortName.getName(),'qualifiedName'=>qualified.getName()};
 }
 public static Map<String,Object> observe() {
  return new Map<String,Object>{
   'fieldResult'=>RuntimeSchemaAliasExternal.fieldPair(),
   'sobjectResult'=>pair(DescribeSObjectResult.class,Schema.DescribeSObjectResult.class),
   'sobjectType'=>pair(SObjectType.class,Schema.SObjectType.class),
   'sobjectField'=>pair(SObjectField.class,Schema.SObjectField.class),
   'primitiveDistinct'=>String.class!=Integer.class,
   'lookupShortField'=>Type.forName('DescribeFieldResult')==Schema.DescribeFieldResult.class,
   'lookupQualifiedField'=>Type.forName('Schema.DescribeFieldResult')==Schema.DescribeFieldResult.class,
   'genericResult'=>List<DescribeSObjectResult>.class==List<Schema.DescribeSObjectResult>.class,
   'distinct'=>Schema.DescribeFieldResult.class!=Schema.DescribeSObjectResult.class,
   'nestedDistinct'=>DescribeFieldResult.class!=Schema.DescribeFieldResult.class,
   'nullDistinct'=>Schema.DescribeFieldResult.class!=null
  };
 }
}
