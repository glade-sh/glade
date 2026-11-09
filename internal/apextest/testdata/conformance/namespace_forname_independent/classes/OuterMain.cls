public class OuterMain {
  private class PrivInner implements Callable {
    public Integer f;
    public Object call(String a, Map<String,Object> m) { return 'PrivInner'; }
  public String ob(Integer s, String o, String k) {
    Type t;
    try {
      if (s == 1) t = Type.forName('gladens004', o + '.' + k);
      else if (s == 2) t = Type.forName('gladens004.' + o + '.' + k);
      else if (s == 3) t = Type.forName(o + '.' + k);
      else if (s == 4) t = Type.forName(null, o + '.' + k);
      else if (s == 5) t = Type.forName('gladens004.' + o + '$' + k);
      else t = Type.forName('', o + '.' + k);
    } catch (Exception e) { return ('EXC:' + e.getTypeName() + ':' + e.getMessage()).replace('\n', ' '); }
    if (t == null) return 'null';
    String r = t.getName();
    try { Object x = t.newInstance(); r += '|ni=ok'; try { Callable c = (Callable) x; r += '|cast=ok:' + String.valueOf(c.call('go', null)); } catch (Exception e) { r += '|cast=' + e.getMessage(); } } catch (Exception e) { r += '|ni=' + e.getMessage(); }
    return r.replace('\n', ' ');
  }
  }
  class DefInner implements Callable {
    public Integer f;
    public Object call(String a, Map<String,Object> m) { return 'DefInner'; }
  }
  @TestVisible
  private class TvInner implements Callable {
    public Integer f;
    public Object call(String a, Map<String,Object> m) { return 'TvInner'; }
  }
  public class PubInner implements Callable {
    public Integer f;
    public Object call(String a, Map<String,Object> m) { return 'PubInner'; }
  public String ob(Integer s, String o, String k) {
    Type t;
    try {
      if (s == 1) t = Type.forName('gladens004', o + '.' + k);
      else if (s == 2) t = Type.forName('gladens004.' + o + '.' + k);
      else if (s == 3) t = Type.forName(o + '.' + k);
      else if (s == 4) t = Type.forName(null, o + '.' + k);
      else if (s == 5) t = Type.forName('gladens004.' + o + '$' + k);
      else t = Type.forName('', o + '.' + k);
    } catch (Exception e) { return ('EXC:' + e.getTypeName() + ':' + e.getMessage()).replace('\n', ' '); }
    if (t == null) return 'null';
    String r = t.getName();
    try { Object x = t.newInstance(); r += '|ni=ok'; try { Callable c = (Callable) x; r += '|cast=ok:' + String.valueOf(c.call('go', null)); } catch (Exception e) { r += '|cast=' + e.getMessage(); } } catch (Exception e) { r += '|ni=' + e.getMessage(); }
    return r.replace('\n', ' ');
  }
  }
  public static String ob_s(Integer s, String o, String k) {
    Type t;
    try {
      if (s == 1) t = Type.forName('gladens004', o + '.' + k);
      else if (s == 2) t = Type.forName('gladens004.' + o + '.' + k);
      else if (s == 3) t = Type.forName(o + '.' + k);
      else if (s == 4) t = Type.forName(null, o + '.' + k);
      else if (s == 5) t = Type.forName('gladens004.' + o + '$' + k);
      else t = Type.forName('', o + '.' + k);
    } catch (Exception e) { return ('EXC:' + e.getTypeName() + ':' + e.getMessage()).replace('\n', ' '); }
    if (t == null) return 'null';
    String r = t.getName();
    try { Object x = t.newInstance(); r += '|ni=ok'; try { Callable c = (Callable) x; r += '|cast=ok:' + String.valueOf(c.call('go', null)); } catch (Exception e) { r += '|cast=' + e.getMessage(); } } catch (Exception e) { r += '|ni=' + e.getMessage(); }
    return r.replace('\n', ' ');
  }
  static Map<String,String> sinit = new Map<String,String>();
  Map<String,String> cinit = new Map<String,String>();
  static {
    sinit.put('1_pr', ob_s(1, 'OuterMain', 'PrivInner'));
    sinit.put('2_pr', ob_s(2, 'OuterMain', 'PrivInner'));
    sinit.put('3_pr', ob_s(3, 'OuterMain', 'PrivInner'));
    sinit.put('4_pr', ob_s(4, 'OuterMain', 'PrivInner'));
    sinit.put('5_pr', ob_s(5, 'OuterMain', 'PrivInner'));
    sinit.put('6_pr', ob_s(6, 'OuterMain', 'PrivInner'));
    sinit.put('1_df', ob_s(1, 'OuterMain', 'DefInner'));
    sinit.put('2_df', ob_s(2, 'OuterMain', 'DefInner'));
    sinit.put('3_df', ob_s(3, 'OuterMain', 'DefInner'));
    sinit.put('4_df', ob_s(4, 'OuterMain', 'DefInner'));
    sinit.put('5_df', ob_s(5, 'OuterMain', 'DefInner'));
    sinit.put('6_df', ob_s(6, 'OuterMain', 'DefInner'));
    sinit.put('1_tv', ob_s(1, 'OuterMain', 'TvInner'));
    sinit.put('2_tv', ob_s(2, 'OuterMain', 'TvInner'));
    sinit.put('3_tv', ob_s(3, 'OuterMain', 'TvInner'));
    sinit.put('4_tv', ob_s(4, 'OuterMain', 'TvInner'));
    sinit.put('5_tv', ob_s(5, 'OuterMain', 'TvInner'));
    sinit.put('6_tv', ob_s(6, 'OuterMain', 'TvInner'));
    sinit.put('1_pu', ob_s(1, 'OuterMain', 'PubInner'));
    sinit.put('2_pu', ob_s(2, 'OuterMain', 'PubInner'));
    sinit.put('3_pu', ob_s(3, 'OuterMain', 'PubInner'));
    sinit.put('4_pu', ob_s(4, 'OuterMain', 'PubInner'));
    sinit.put('5_pu', ob_s(5, 'OuterMain', 'PubInner'));
    sinit.put('6_pu', ob_s(6, 'OuterMain', 'PubInner'));
  }
  public OuterMain() {
    cinit.put('1_pr', ob_s(1, 'OuterMain', 'PrivInner'));
    cinit.put('2_pr', ob_s(2, 'OuterMain', 'PrivInner'));
    cinit.put('3_pr', ob_s(3, 'OuterMain', 'PrivInner'));
    cinit.put('4_pr', ob_s(4, 'OuterMain', 'PrivInner'));
    cinit.put('5_pr', ob_s(5, 'OuterMain', 'PrivInner'));
    cinit.put('6_pr', ob_s(6, 'OuterMain', 'PrivInner'));
    cinit.put('1_df', ob_s(1, 'OuterMain', 'DefInner'));
    cinit.put('2_df', ob_s(2, 'OuterMain', 'DefInner'));
    cinit.put('3_df', ob_s(3, 'OuterMain', 'DefInner'));
    cinit.put('4_df', ob_s(4, 'OuterMain', 'DefInner'));
    cinit.put('5_df', ob_s(5, 'OuterMain', 'DefInner'));
    cinit.put('6_df', ob_s(6, 'OuterMain', 'DefInner'));
    cinit.put('1_tv', ob_s(1, 'OuterMain', 'TvInner'));
    cinit.put('2_tv', ob_s(2, 'OuterMain', 'TvInner'));
    cinit.put('3_tv', ob_s(3, 'OuterMain', 'TvInner'));
    cinit.put('4_tv', ob_s(4, 'OuterMain', 'TvInner'));
    cinit.put('5_tv', ob_s(5, 'OuterMain', 'TvInner'));
    cinit.put('6_tv', ob_s(6, 'OuterMain', 'TvInner'));
    cinit.put('1_pu', ob_s(1, 'OuterMain', 'PubInner'));
    cinit.put('2_pu', ob_s(2, 'OuterMain', 'PubInner'));
    cinit.put('3_pu', ob_s(3, 'OuterMain', 'PubInner'));
    cinit.put('4_pu', ob_s(4, 'OuterMain', 'PubInner'));
    cinit.put('5_pu', ob_s(5, 'OuterMain', 'PubInner'));
    cinit.put('6_pu', ob_s(6, 'OuterMain', 'PubInner'));
  }
  public static String sinitGet(String k) { return sinit.get(k); }
  public static String cinitGet(String k) { return new OuterMain().cinit.get(k); }
  public static String viaPub(Integer s, String o, String k) { return new PubInner().ob(s, o, k); }
  public static String viaPriv(Integer s, String o, String k) { return new PrivInner().ob(s, o, k); }
}
