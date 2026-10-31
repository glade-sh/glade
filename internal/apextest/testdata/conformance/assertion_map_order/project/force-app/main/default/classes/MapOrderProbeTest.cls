@IsTest private class MapOrderProbeTest {
 @IsTest static void observed() {
  Map<String,Object> a=new Map<String,Object>(); a.put('alpha',1); a.put('beta',2);
  System.assertEquals('{"beta":2,"alpha":1}',JSON.serialize(a));
  Map<String,Object> b=new Map<String,Object>(); b.put('beta',2); b.put('alpha',1);
  System.assertEquals('{"alpha":1,"beta":2}',JSON.serialize(b));
  Map<String,Object> c=new Map<String,Object>(); c.put('inner',a);
  System.assertEquals('{"inner":{"beta":2,"alpha":1}}',JSON.serialize(c));
  Map<String,Object> d=new Map<String,Object>(); d.put('alpha',1);
  System.assertEquals('{"alpha":1}',JSON.serialize(d));
  d.put(null,1); Boolean rejected=false;
  try { JSON.serialize(d); } catch(JSONException e) { rejected=true; }
  System.assertEquals(true,rejected);
 }
}
