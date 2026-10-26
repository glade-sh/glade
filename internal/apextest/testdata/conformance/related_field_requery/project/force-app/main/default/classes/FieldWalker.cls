public class FieldWalker {
 public static Boolean present(SObject row, List<String> path) {
  List<String> remaining = new List<String>(path);
  try {
   if(remaining.size()>1) {
    SObject parent = row.getSObject(remaining.remove(0));
    return parent != null && present(parent,remaining);
   }
   return row.get(remaining[0]) != null;
  } catch(SObjectException e) { return false; }
 }
}
