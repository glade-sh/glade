public class StdProbe {
 public static String assignment() {
  String result = 'self=' + String.valueOf(String.class.isAssignableFrom(String.class));
  result += ';other=' + String.valueOf(String.class.isAssignableFrom(Integer.class));
  try { result += ';null=' + String.valueOf(String.class.isAssignableFrom(null)); }
  catch (Exception e) { result += ';null=' + e.getTypeName() + ':' + e.getMessage(); }
  return result;
 }
 public static Integer depth(Integer n) { if (n == 0) return 0; return 1 + depth(n - 1); }
}
