public class HeapIdentity67 {
 public static String observe() {
  Integer before = Limits.getHeapSize();
  HeapPayload held = new HeapPayload();
  held.value = 'x'.repeat(100000);
  Boolean counted = Limits.getHeapSize() - before >= 100000;
  String invalid;
  try { User u = new User(Id='005-community-user'); invalid='accepted'; }
  catch(Exception e) { invalid=e.getTypeName()+'|'+e.getMessage(); }
  User valid = new User(Id='005000000000071');
  return JSON.serialize(new Map<String,Object>{'userBlobCounted'=>counted,'invalidUser'=>invalid,'validUser'=>String.valueOf(valid.Id)});
 }
}