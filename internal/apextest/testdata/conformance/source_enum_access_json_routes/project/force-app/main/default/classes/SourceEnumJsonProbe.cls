public class SourceEnumJsonProbe {
 public enum State { READY, DONE }
 public class Envelope { public Object value; public Envelope(Object item) { value=item; } }
 public static Map<String,Object> observe(String shape) {
  Object item; if(shape=='shadow'){item=AccessLevel.LOCAL;}else{item=State.READY;}
  List<Object> values=new List<Object>{item,new Envelope(item),new List<Object>{item},new Map<String,Object>{'value'=>item}};
  List<String> names=new List<String>{'direct','envelope','list','map'};
  Map<String,Object> results=new Map<String,Object>();
  Object twin; if(shape=='shadow'){twin=AccessLevel.LOCAL;}else{twin=State.READY;}
  results.put('equal',item==twin);results.put('same',item===twin);results.put('equals',item.equals(twin));
  results.put('listContains',new List<Object>{item}.contains(twin));
  results.put('setContains',new Set<Object>{item}.contains(twin));
  results.put('mapLookup',new Map<Object,String>{item=>'found'}.get(twin));
  for(Integer i=0;i<values.size();i++) {
   Object value=values[i]; String name=names[i];
   results.put(name+'Compact',JSON.serialize(value));
   results.put(name+'Suppress',JSON.serialize(value,true));
   results.put(name+'Pretty',JSON.serializePretty(value));
   results.put(name+'PrettyNulls',JSON.serializePretty(value,false));
   JSONGenerator g=JSON.createGenerator(false);g.writeObject(value);results.put(name+'Generator',g.getAsString());
   g=JSON.createGenerator(false);g.writeStartObject();g.writeObjectField('value',value);g.writeEndObject();results.put(name+'GeneratorField',g.getAsString());
  }
  return results;
 }
}
