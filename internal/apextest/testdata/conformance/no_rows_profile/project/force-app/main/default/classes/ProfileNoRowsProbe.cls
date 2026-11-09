public class ProfileNoRowsProbe {
 public static String sample() {
  Map<String,Object> out=new Map<String,Object>();
  out.put('existing',[SELECT COUNT() FROM Profile WHERE Name='Standard User']);
  out.put('missing',[SELECT COUNT() FROM Profile WHERE Name='Synthetic Absent Profile']);
  try { Profile p=[SELECT Id FROM Profile WHERE Name='Synthetic Absent Profile']; out.put('singleton','found'); }
  catch(QueryException e) { out.put('singleton',e.getTypeName()); }
  return JSON.serialize(out);
 }
}
