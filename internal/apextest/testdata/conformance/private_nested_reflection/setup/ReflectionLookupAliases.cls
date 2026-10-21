public class ReflectionLookupAliases {
 public static Map<String,Object> observe() {
  Map<String,Object> rows=new Map<String,Object>();
  for(String name:new List<String>{'ReflectionSubjects.PrivateChild','ReflectionSubjects.PublicChild'}) {
   String prefix=name.substringBefore('.')+'.';
   Map<String,Object> row=new Map<String,Object>{'namespaceArgument'=>prefix,'nameArgument'=>name};
   try {Type found=Type.forName(prefix,name);row.put('found',found!=null);if(found!=null)row.put('name',found.getName());}
   catch(Exception failure) {row.put('errorType',failure.getTypeName());row.put('error',failure.getMessage());}
   rows.put(name.substringAfter('.'),row);
  }
  return rows;
 }
}
