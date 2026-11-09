public class NsVis {
  class Dn { public Dn(){} }
  private class Pv { public Pv(){} }
  @TestVisible private class Tv { public Tv(){} }
  public class Pb { public Pb(){} }
  public static String fn(String ns, String n) { Type t = Type.forName(ns, n); return t == null ? 'null' : t.getName(); }
  public static String f1(String n) { Type t = Type.forName(n); return t == null ? 'null' : t.getName(); }
  public static String inst(String n) { Type t = Type.forName(n); if (t == null) return 'null'; try { Object o = t.newInstance(); return 'ok'; } catch (Exception e) { return e.getTypeName(); } }
}
