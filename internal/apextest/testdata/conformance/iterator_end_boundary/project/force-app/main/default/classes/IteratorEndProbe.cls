public class IteratorEndProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> out = new Map<String,Object>();
  Set<String> single = new Set<String>{'alpha'};
  String first = single.iterator().next();
  try { out.put('setLastRemoved',single.remove(first)); } catch(Exception e) { out.put('setLastError',e.getTypeName()+':'+e.getMessage()); }
  out.put('setLastSize',single.size());
  Set<String> pair = new Set<String>{'alpha','beta'};
  Iterator<String> it = pair.iterator(); String selected=it.next();
  try { out.put('setPartialRemoved',pair.remove(selected)); } catch(Exception e) { out.put('setPartialError',e.getTypeName()+':'+e.getMessage()); }
  out.put('setPartialSize',pair.size());
  Set<String> completed = new Set<String>{'gamma'}; Iterator<String> tailCursor=completed.iterator(); tailCursor.next(); Boolean more=tailCursor.hasNext();
  try { out.put('setCheckedRemoved',completed.remove('gamma')); } catch(Exception e) { out.put('setCheckedError',e.getTypeName()+':'+e.getMessage()); }
  List<String> listOne = new List<String>{'alpha'}; String item=listOne.iterator().next();
  try { out.put('listLastRemoved',listOne.remove(0)); } catch(Exception e) { out.put('listLastError',e.getTypeName()+':'+e.getMessage()); }
  out.put('listLastSize',listOne.size());

  return out;
 }
}
