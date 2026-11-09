public class N24 {
  public static String go(String n) { Type t = Type.forName(n); return t == null ? 'null' : t.getName(); }
}
