public class AccessJsonProbe {
 public class Envelope { public AccessLevel level; public Envelope(AccessLevel value) {level=value;} }
 public static Map<String,Object> observe(String shape) {
  AccessLevel level=shape=='jsonSystem'||shape=='identitySystem'?AccessLevel.SYSTEM_MODE:AccessLevel.USER_MODE;
  Map<String,Object> results=new Map<String,Object>();
  if(shape=='identityUser'||shape=='identitySystem') {
   AccessLevel copy=(AccessLevel)level.clone(); AccessLevel alias=copy;
   results.put('constantReference',level===(shape=='identitySystem'?AccessLevel.SYSTEM_MODE:AccessLevel.USER_MODE));
   results.put('cloneUnequal',level!=copy);results.put('cloneNotIdentical',level!==copy);
   results.put('cloneAliasEqual',copy.equals(alias));results.put('cloneCloneEqual',copy.equals(copy.clone()));
   results.put('cloneNullEqual',copy==null);results.put('constantNullEqual',level.equals(null));
   results.put('listClone',new List<AccessLevel>{level}.contains(copy));results.put('listAlias',new List<AccessLevel>{level}.contains(level));
   results.put('setClone',new Set<AccessLevel>{level}.contains(copy));results.put('setAlias',new Set<AccessLevel>{level}.contains(level));
   results.put('mapClone',new Map<AccessLevel,String>{level=>'found'}.get(copy));results.put('mapAlias',new Map<AccessLevel,String>{level=>'found'}.get(level));
   for(String route:new List<String>{'instance'}) {
    try {
     AccessLevel first=level.withPermissionSetId(null);
     AccessLevel second=level.withPermissionSetId(null);
     results.put(route+'NullFactoryEqual',first==second);results.put(route+'NullFactoryConstant',first==level);
    }catch(Exception e){results.put(route+'NullFactoryError',e.getTypeName());}
   }
   return results;
  }
  List<Object> values=new List<Object>{level,new Envelope(level),new List<AccessLevel>{level},new Map<String,Object>{'level'=>level}};
  List<String> names=new List<String>{'direct','envelope','list','map'};
  for(Integer i=0;i<values.size();i++) {
   Object value=values[i];String name=names[i];
   try {JSON.serialize(value);results.put(name+'Compact','accepted');}catch(Exception e){results.put(name+'Compact',e.getTypeName()+':'+e.getMessage());}
   try {JSON.serialize(value,true);results.put(name+'Suppress','accepted');}catch(Exception e){results.put(name+'Suppress',e.getTypeName()+':'+e.getMessage());}
   try {JSON.serializePretty(value);results.put(name+'Pretty','accepted');}catch(Exception e){results.put(name+'Pretty',e.getTypeName()+':'+e.getMessage());}
   try {JSON.serializePretty(value,false);results.put(name+'PrettyNulls','accepted');}catch(Exception e){results.put(name+'PrettyNulls',e.getTypeName()+':'+e.getMessage());}
   try {JSONGenerator g=JSON.createGenerator(false);g.writeObject(value);results.put(name+'Generator','accepted');}catch(Exception e){results.put(name+'Generator',e.getTypeName()+':'+e.getMessage());}
   try {JSONGenerator g=JSON.createGenerator(true);g.writeStartObject();g.writeObjectField('value',value);g.writeEndObject();results.put(name+'GeneratorField','accepted');}catch(Exception e){results.put(name+'GeneratorField',e.getTypeName()+':'+e.getMessage());}
  }
  return results;
 }
}
