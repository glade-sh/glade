public class BoundaryProbe {
 public static String variable() {
  Type receiver = String.class;
  Type good = String.class;
  Type missing = Type.forName('DefinitelyAbsentType');
  String result = 'same=' + String.valueOf(receiver.isAssignableFrom(good));
  try { result += ';null=' + String.valueOf(receiver.isAssignableFrom(missing)); }
  catch (Exception e) { result += ';null=' + e.getTypeName() + ':' + e.getMessage(); }
  return result;
 }
 public static Integer depth(Integer n) { if (n == 0) return 0; return 1 + depth(n - 1); }
}
