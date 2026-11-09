public class ProbeMore2712 {
 public static Map<String,Object> observe(String name) {
  Map<String,Object> out=new Map<String,Object>();
  try {Contact c=(Contact)JSON.deserializeStrict('{"account":{"Name":"lower"}}',Contact.class);out.put('strictLower',c.Account==null?null:c.Account.Name);}catch(Exception e){out.put('strictLower',e.getTypeName());}
  try {Contact c=(Contact)JSON.deserializeStrict('{"Account":{"Name":"upper"}}',Contact.class);out.put('strictExact',c.Account==null?null:c.Account.Name);}catch(Exception e){out.put('strictExact',e.getTypeName());}
  try {Contact c=(Contact)JSON.deserialize('{"attributes":{"type":"Contact"},"account":{"attributes":{"type":"Account"},"Name":"lower"}}',SObject.class);out.put('genericLower',c.Account==null?null:c.Account.Name);}catch(Exception e){out.put('genericLower',e.getTypeName());}
  try {Contact c=(Contact)JSON.deserialize('{"attributes":{"type":"Contact"},"Account":{"attributes":{"type":"Account"},"Name":"upper"}}',SObject.class);out.put('genericExact',c.Account==null?null:c.Account.Name);}catch(Exception e){out.put('genericExact',e.getTypeName());}
  return out;
 }
}
