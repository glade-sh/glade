public class V035Vis {
  class Dn { public Dn(){} public Integer y(){return 7;} }
  private class Pv { public Pv(){} public Integer y(){return 8;} }
  @TestVisible private class Tv { public Tv(){} public Integer y(){return 9;} }
  @TestVisible class Tdn { public Tdn(){} public Integer y(){return 10;} }
  private enum Pen { A, B }
  private interface Pi { Integer f(); }
  public class Pb { public Pb(){} public Integer y(){return 11;} }
  public static Integer inner() { Pv p = new Pv(); Dn d = new Dn(); Pen e = Pen.B; return p.y() + d.y(); }
  public static String fn(String n) { Type t = Type.forName(n); return t == null ? 'null' : t.getName(); }
  public class Sib { public Integer z() { Pv p = new Pv(); Dn d = new Dn(); return p.y() + d.y(); } }
}
