public class ReflectionLookupOwner {
 private class Hidden {}
 public static Map<String,Object> observe() {
  Map<String,Object> row=new Map<String,Object>{
   'singleFound'=>Type.forName('ReflectionLookupOwner.Hidden')!=null,
   'emptyNamespaceFound'=>Type.forName('','ReflectionLookupOwner.Hidden')!=null,
   'literalName'=>Hidden.class.getName(),
   'directConstruction'=>new Hidden()!=null
  };
  try {row.put('literalConstruction',Hidden.class.newInstance()!=null);} catch(Exception e) {row.put('literalConstructionError',e.getTypeName()+': '+e.getMessage());}
  try {row.put('literalDeserialize',JSON.deserialize('{}',Hidden.class)!=null);} catch(Exception e) {row.put('literalDeserializeError',e.getTypeName()+': '+e.getMessage());}
  return row;
 }
}
