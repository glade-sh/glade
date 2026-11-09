@IsTest private class NsVisTest {
  private class TPv { public TPv(){} }
  public class TPb { public TPb(){} }
  static String o(String ns, String n) { return NsVis.fn(ns, n); }
  static String t(String ns, String n) { Type x = Type.forName(ns, n); return x == null ? 'null' : x.getName(); }
  static String s(String n) { Type x = Type.forName(n); return x == null ? 'null' : x.getName(); }
  @IsTest static void m() {
    List<String> r = new List<String>();
    for (String k : new List<String>{'Pb','Pv','Dn','Tv'}) {
      r.add('OBS|d1_' + k + '|ns,Outer.k|' + t('gladens004', 'NsVis.' + k));
      r.add('OBS|d2_' + k + '|ns.Outer.k|' + s('gladens004.NsVis.' + k));
      r.add('OBS|d3_' + k + '|null,Outer.k|' + t(null, 'NsVis.' + k));
      r.add('OBS|d4_' + k + '|Outer.k|' + s('NsVis.' + k));
      r.add('OBS|d5_' + k + '|ns.Outer$k|' + s('gladens004.NsVis$' + k));
      r.add('OBS|d6_' + k + '|helper ns,Outer.k|' + o('gladens004', 'NsVis.' + k));
      r.add('OBS|d7_' + k + '|helper f1 ns.Outer.k|' + NsVis.f1('gladens004.NsVis.' + k));
      r.add('OBS|d8_' + k + '|inst ns.Outer.k|' + NsVis.inst('gladens004.NsVis.' + k));
    }
    r.add('OBS|e1|self ns,Test.TPv|' + t('gladens004', 'NsVisTest.TPv'));
    r.add('OBS|e2|self ns.Test.TPv|' + s('gladens004.NsVisTest.TPv'));
    r.add('OBS|e3|self Test.TPv|' + s('NsVisTest.TPv'));
    r.add('OBS|e4|self ns,Test.TPb|' + t('gladens004', 'NsVisTest.TPb'));
    r.add('OBS|e5|self ns.Test.TPv$|' + s('gladens004.NsVisTest$TPv'));
    r.add('OBS|e6|helper ns,Test.TPv|' + o('gladens004', 'NsVisTest.TPv'));
    r.add('OBS|e7|inst ns.Test.TPv|' + NsVis.inst('gladens004.NsVisTest.TPv'));
    r.add('OBS|e8|self lit TPv|' + TPv.class.getName());
    System.assert(false, String.join(r, '\n'));
  }
}
