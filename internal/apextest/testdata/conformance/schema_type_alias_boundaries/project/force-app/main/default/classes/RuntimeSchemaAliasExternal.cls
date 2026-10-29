public class RuntimeSchemaAliasExternal {
 public static Map<String,Object> fieldPair(){
  Type shortName=DescribeFieldResult.class, qualified=Schema.DescribeFieldResult.class;
  return new Map<String,Object>{'equal'=>shortName==qualified,'methodEqual'=>shortName.equals(qualified),'setSize'=>new Set<Type>{shortName,qualified}.size(),'mapLookup'=>new Map<Type,String>{shortName=>'retained'}.get(qualified),'shortName'=>shortName.getName(),'qualifiedName'=>qualified.getName()};
 }
}
