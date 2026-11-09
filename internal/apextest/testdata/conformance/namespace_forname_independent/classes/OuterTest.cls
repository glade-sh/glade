@IsTest
private class OuterTest {
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
  static String jd(Type t) {
    try { Object o = JSON.deserialize('{"f":7}', t); return 'ok:' + String.valueOf(o).substringBefore(':'); } catch (Exception e) { return ('EXC:' + e.getTypeName() + ':' + e.getMessage()).replace('\n', ' '); }
  }
  static String jf(String n) {
    try { Type t = Type.forName(n); return jd(t); } catch (Exception e) { return ('EXC:' + e.getTypeName() + ':' + e.getMessage()).replace('\n', ' '); }
  }
  static Map<String,String> sinit = new Map<String,String>();
  Map<String,String> cinit = new Map<String,String>();
  static {
    sinit.put('1_pr', ob_s(1, 'OuterTest', 'PrivInner'));
    sinit.put('2_pr', ob_s(2, 'OuterTest', 'PrivInner'));
    sinit.put('3_pr', ob_s(3, 'OuterTest', 'PrivInner'));
    sinit.put('4_pr', ob_s(4, 'OuterTest', 'PrivInner'));
    sinit.put('5_pr', ob_s(5, 'OuterTest', 'PrivInner'));
    sinit.put('6_pr', ob_s(6, 'OuterTest', 'PrivInner'));
    sinit.put('1_df', ob_s(1, 'OuterTest', 'DefInner'));
    sinit.put('2_df', ob_s(2, 'OuterTest', 'DefInner'));
    sinit.put('3_df', ob_s(3, 'OuterTest', 'DefInner'));
    sinit.put('4_df', ob_s(4, 'OuterTest', 'DefInner'));
    sinit.put('5_df', ob_s(5, 'OuterTest', 'DefInner'));
    sinit.put('6_df', ob_s(6, 'OuterTest', 'DefInner'));
    sinit.put('1_tv', ob_s(1, 'OuterTest', 'TvInner'));
    sinit.put('2_tv', ob_s(2, 'OuterTest', 'TvInner'));
    sinit.put('3_tv', ob_s(3, 'OuterTest', 'TvInner'));
    sinit.put('4_tv', ob_s(4, 'OuterTest', 'TvInner'));
    sinit.put('5_tv', ob_s(5, 'OuterTest', 'TvInner'));
    sinit.put('6_tv', ob_s(6, 'OuterTest', 'TvInner'));
    sinit.put('1_pu', ob_s(1, 'OuterTest', 'PubInner'));
    sinit.put('2_pu', ob_s(2, 'OuterTest', 'PubInner'));
    sinit.put('3_pu', ob_s(3, 'OuterTest', 'PubInner'));
    sinit.put('4_pu', ob_s(4, 'OuterTest', 'PubInner'));
    sinit.put('5_pu', ob_s(5, 'OuterTest', 'PubInner'));
    sinit.put('6_pu', ob_s(6, 'OuterTest', 'PubInner'));
  }
  public OuterTest() {
    cinit.put('1_pr', ob_s(1, 'OuterTest', 'PrivInner'));
    cinit.put('2_pr', ob_s(2, 'OuterTest', 'PrivInner'));
    cinit.put('3_pr', ob_s(3, 'OuterTest', 'PrivInner'));
    cinit.put('4_pr', ob_s(4, 'OuterTest', 'PrivInner'));
    cinit.put('5_pr', ob_s(5, 'OuterTest', 'PrivInner'));
    cinit.put('6_pr', ob_s(6, 'OuterTest', 'PrivInner'));
    cinit.put('1_df', ob_s(1, 'OuterTest', 'DefInner'));
    cinit.put('2_df', ob_s(2, 'OuterTest', 'DefInner'));
    cinit.put('3_df', ob_s(3, 'OuterTest', 'DefInner'));
    cinit.put('4_df', ob_s(4, 'OuterTest', 'DefInner'));
    cinit.put('5_df', ob_s(5, 'OuterTest', 'DefInner'));
    cinit.put('6_df', ob_s(6, 'OuterTest', 'DefInner'));
    cinit.put('1_tv', ob_s(1, 'OuterTest', 'TvInner'));
    cinit.put('2_tv', ob_s(2, 'OuterTest', 'TvInner'));
    cinit.put('3_tv', ob_s(3, 'OuterTest', 'TvInner'));
    cinit.put('4_tv', ob_s(4, 'OuterTest', 'TvInner'));
    cinit.put('5_tv', ob_s(5, 'OuterTest', 'TvInner'));
    cinit.put('6_tv', ob_s(6, 'OuterTest', 'TvInner'));
    cinit.put('1_pu', ob_s(1, 'OuterTest', 'PubInner'));
    cinit.put('2_pu', ob_s(2, 'OuterTest', 'PubInner'));
    cinit.put('3_pu', ob_s(3, 'OuterTest', 'PubInner'));
    cinit.put('4_pu', ob_s(4, 'OuterTest', 'PubInner'));
    cinit.put('5_pu', ob_s(5, 'OuterTest', 'PubInner'));
    cinit.put('6_pu', ob_s(6, 'OuterTest', 'PubInner'));
  }
  @IsTest static void m_i_1_ot_pr() { System.assert(false, 'OBS|i_1_ot_pr|' + ob_s(1, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_i_2_ot_pr() { System.assert(false, 'OBS|i_2_ot_pr|' + ob_s(2, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_i_3_ot_pr() { System.assert(false, 'OBS|i_3_ot_pr|' + ob_s(3, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_i_4_ot_pr() { System.assert(false, 'OBS|i_4_ot_pr|' + ob_s(4, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_i_5_ot_pr() { System.assert(false, 'OBS|i_5_ot_pr|' + ob_s(5, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_i_6_ot_pr() { System.assert(false, 'OBS|i_6_ot_pr|' + ob_s(6, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_i_1_ot_df() { System.assert(false, 'OBS|i_1_ot_df|' + ob_s(1, 'OuterTest', 'DefInner')); }
  @IsTest static void m_i_2_ot_df() { System.assert(false, 'OBS|i_2_ot_df|' + ob_s(2, 'OuterTest', 'DefInner')); }
  @IsTest static void m_i_3_ot_df() { System.assert(false, 'OBS|i_3_ot_df|' + ob_s(3, 'OuterTest', 'DefInner')); }
  @IsTest static void m_i_4_ot_df() { System.assert(false, 'OBS|i_4_ot_df|' + ob_s(4, 'OuterTest', 'DefInner')); }
  @IsTest static void m_i_5_ot_df() { System.assert(false, 'OBS|i_5_ot_df|' + ob_s(5, 'OuterTest', 'DefInner')); }
  @IsTest static void m_i_6_ot_df() { System.assert(false, 'OBS|i_6_ot_df|' + ob_s(6, 'OuterTest', 'DefInner')); }
  @IsTest static void m_i_1_ot_tv() { System.assert(false, 'OBS|i_1_ot_tv|' + ob_s(1, 'OuterTest', 'TvInner')); }
  @IsTest static void m_i_2_ot_tv() { System.assert(false, 'OBS|i_2_ot_tv|' + ob_s(2, 'OuterTest', 'TvInner')); }
  @IsTest static void m_i_3_ot_tv() { System.assert(false, 'OBS|i_3_ot_tv|' + ob_s(3, 'OuterTest', 'TvInner')); }
  @IsTest static void m_i_4_ot_tv() { System.assert(false, 'OBS|i_4_ot_tv|' + ob_s(4, 'OuterTest', 'TvInner')); }
  @IsTest static void m_i_5_ot_tv() { System.assert(false, 'OBS|i_5_ot_tv|' + ob_s(5, 'OuterTest', 'TvInner')); }
  @IsTest static void m_i_6_ot_tv() { System.assert(false, 'OBS|i_6_ot_tv|' + ob_s(6, 'OuterTest', 'TvInner')); }
  @IsTest static void m_i_1_ot_pu() { System.assert(false, 'OBS|i_1_ot_pu|' + ob_s(1, 'OuterTest', 'PubInner')); }
  @IsTest static void m_i_2_ot_pu() { System.assert(false, 'OBS|i_2_ot_pu|' + ob_s(2, 'OuterTest', 'PubInner')); }
  @IsTest static void m_i_3_ot_pu() { System.assert(false, 'OBS|i_3_ot_pu|' + ob_s(3, 'OuterTest', 'PubInner')); }
  @IsTest static void m_i_4_ot_pu() { System.assert(false, 'OBS|i_4_ot_pu|' + ob_s(4, 'OuterTest', 'PubInner')); }
  @IsTest static void m_i_5_ot_pu() { System.assert(false, 'OBS|i_5_ot_pu|' + ob_s(5, 'OuterTest', 'PubInner')); }
  @IsTest static void m_i_6_ot_pu() { System.assert(false, 'OBS|i_6_ot_pu|' + ob_s(6, 'OuterTest', 'PubInner')); }
  @IsTest static void m_i_1_ou_pr() { System.assert(false, 'OBS|i_1_ou_pr|' + ob_s(1, 'OuterMain', 'PrivInner')); }
  @IsTest static void m_i_2_ou_pr() { System.assert(false, 'OBS|i_2_ou_pr|' + ob_s(2, 'OuterMain', 'PrivInner')); }
  @IsTest static void m_i_3_ou_pr() { System.assert(false, 'OBS|i_3_ou_pr|' + ob_s(3, 'OuterMain', 'PrivInner')); }
  @IsTest static void m_i_4_ou_pr() { System.assert(false, 'OBS|i_4_ou_pr|' + ob_s(4, 'OuterMain', 'PrivInner')); }
  @IsTest static void m_i_5_ou_pr() { System.assert(false, 'OBS|i_5_ou_pr|' + ob_s(5, 'OuterMain', 'PrivInner')); }
  @IsTest static void m_i_6_ou_pr() { System.assert(false, 'OBS|i_6_ou_pr|' + ob_s(6, 'OuterMain', 'PrivInner')); }
  @IsTest static void m_i_1_ou_df() { System.assert(false, 'OBS|i_1_ou_df|' + ob_s(1, 'OuterMain', 'DefInner')); }
  @IsTest static void m_i_2_ou_df() { System.assert(false, 'OBS|i_2_ou_df|' + ob_s(2, 'OuterMain', 'DefInner')); }
  @IsTest static void m_i_3_ou_df() { System.assert(false, 'OBS|i_3_ou_df|' + ob_s(3, 'OuterMain', 'DefInner')); }
  @IsTest static void m_i_4_ou_df() { System.assert(false, 'OBS|i_4_ou_df|' + ob_s(4, 'OuterMain', 'DefInner')); }
  @IsTest static void m_i_5_ou_df() { System.assert(false, 'OBS|i_5_ou_df|' + ob_s(5, 'OuterMain', 'DefInner')); }
  @IsTest static void m_i_6_ou_df() { System.assert(false, 'OBS|i_6_ou_df|' + ob_s(6, 'OuterMain', 'DefInner')); }
  @IsTest static void m_i_1_ou_tv() { System.assert(false, 'OBS|i_1_ou_tv|' + ob_s(1, 'OuterMain', 'TvInner')); }
  @IsTest static void m_i_2_ou_tv() { System.assert(false, 'OBS|i_2_ou_tv|' + ob_s(2, 'OuterMain', 'TvInner')); }
  @IsTest static void m_i_3_ou_tv() { System.assert(false, 'OBS|i_3_ou_tv|' + ob_s(3, 'OuterMain', 'TvInner')); }
  @IsTest static void m_i_4_ou_tv() { System.assert(false, 'OBS|i_4_ou_tv|' + ob_s(4, 'OuterMain', 'TvInner')); }
  @IsTest static void m_i_5_ou_tv() { System.assert(false, 'OBS|i_5_ou_tv|' + ob_s(5, 'OuterMain', 'TvInner')); }
  @IsTest static void m_i_6_ou_tv() { System.assert(false, 'OBS|i_6_ou_tv|' + ob_s(6, 'OuterMain', 'TvInner')); }
  @IsTest static void m_i_1_ou_pu() { System.assert(false, 'OBS|i_1_ou_pu|' + ob_s(1, 'OuterMain', 'PubInner')); }
  @IsTest static void m_i_2_ou_pu() { System.assert(false, 'OBS|i_2_ou_pu|' + ob_s(2, 'OuterMain', 'PubInner')); }
  @IsTest static void m_i_3_ou_pu() { System.assert(false, 'OBS|i_3_ou_pu|' + ob_s(3, 'OuterMain', 'PubInner')); }
  @IsTest static void m_i_4_ou_pu() { System.assert(false, 'OBS|i_4_ou_pu|' + ob_s(4, 'OuterMain', 'PubInner')); }
  @IsTest static void m_i_5_ou_pu() { System.assert(false, 'OBS|i_5_ou_pu|' + ob_s(5, 'OuterMain', 'PubInner')); }
  @IsTest static void m_i_6_ou_pu() { System.assert(false, 'OBS|i_6_ou_pu|' + ob_s(6, 'OuterMain', 'PubInner')); }
  @IsTest static void m_np_1_ot_pr() { System.assert(false, 'OBS|np_1_ot_pr|' + new PubInner().ob(1, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_np_2_ot_pr() { System.assert(false, 'OBS|np_2_ot_pr|' + new PubInner().ob(2, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_np_3_ot_pr() { System.assert(false, 'OBS|np_3_ot_pr|' + new PubInner().ob(3, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_np_4_ot_pr() { System.assert(false, 'OBS|np_4_ot_pr|' + new PubInner().ob(4, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_np_5_ot_pr() { System.assert(false, 'OBS|np_5_ot_pr|' + new PubInner().ob(5, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_np_6_ot_pr() { System.assert(false, 'OBS|np_6_ot_pr|' + new PubInner().ob(6, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_np_1_ot_df() { System.assert(false, 'OBS|np_1_ot_df|' + new PubInner().ob(1, 'OuterTest', 'DefInner')); }
  @IsTest static void m_np_2_ot_df() { System.assert(false, 'OBS|np_2_ot_df|' + new PubInner().ob(2, 'OuterTest', 'DefInner')); }
  @IsTest static void m_np_3_ot_df() { System.assert(false, 'OBS|np_3_ot_df|' + new PubInner().ob(3, 'OuterTest', 'DefInner')); }
  @IsTest static void m_np_4_ot_df() { System.assert(false, 'OBS|np_4_ot_df|' + new PubInner().ob(4, 'OuterTest', 'DefInner')); }
  @IsTest static void m_np_5_ot_df() { System.assert(false, 'OBS|np_5_ot_df|' + new PubInner().ob(5, 'OuterTest', 'DefInner')); }
  @IsTest static void m_np_6_ot_df() { System.assert(false, 'OBS|np_6_ot_df|' + new PubInner().ob(6, 'OuterTest', 'DefInner')); }
  @IsTest static void m_np_1_ot_tv() { System.assert(false, 'OBS|np_1_ot_tv|' + new PubInner().ob(1, 'OuterTest', 'TvInner')); }
  @IsTest static void m_np_2_ot_tv() { System.assert(false, 'OBS|np_2_ot_tv|' + new PubInner().ob(2, 'OuterTest', 'TvInner')); }
  @IsTest static void m_np_3_ot_tv() { System.assert(false, 'OBS|np_3_ot_tv|' + new PubInner().ob(3, 'OuterTest', 'TvInner')); }
  @IsTest static void m_np_4_ot_tv() { System.assert(false, 'OBS|np_4_ot_tv|' + new PubInner().ob(4, 'OuterTest', 'TvInner')); }
  @IsTest static void m_np_5_ot_tv() { System.assert(false, 'OBS|np_5_ot_tv|' + new PubInner().ob(5, 'OuterTest', 'TvInner')); }
  @IsTest static void m_np_6_ot_tv() { System.assert(false, 'OBS|np_6_ot_tv|' + new PubInner().ob(6, 'OuterTest', 'TvInner')); }
  @IsTest static void m_np_1_ot_pu() { System.assert(false, 'OBS|np_1_ot_pu|' + new PubInner().ob(1, 'OuterTest', 'PubInner')); }
  @IsTest static void m_np_2_ot_pu() { System.assert(false, 'OBS|np_2_ot_pu|' + new PubInner().ob(2, 'OuterTest', 'PubInner')); }
  @IsTest static void m_np_3_ot_pu() { System.assert(false, 'OBS|np_3_ot_pu|' + new PubInner().ob(3, 'OuterTest', 'PubInner')); }
  @IsTest static void m_np_4_ot_pu() { System.assert(false, 'OBS|np_4_ot_pu|' + new PubInner().ob(4, 'OuterTest', 'PubInner')); }
  @IsTest static void m_np_5_ot_pu() { System.assert(false, 'OBS|np_5_ot_pu|' + new PubInner().ob(5, 'OuterTest', 'PubInner')); }
  @IsTest static void m_np_6_ot_pu() { System.assert(false, 'OBS|np_6_ot_pu|' + new PubInner().ob(6, 'OuterTest', 'PubInner')); }
  @IsTest static void m_np_1_ot_dp() { System.assert(false, 'OBS|np_1_ot_dp|' + new PubInner().ob(1, 'OuterTest.PubInner', 'Deeper')); }
  @IsTest static void m_np_2_ot_dp() { System.assert(false, 'OBS|np_2_ot_dp|' + new PubInner().ob(2, 'OuterTest.PubInner', 'Deeper')); }
  @IsTest static void m_np_3_ot_dp() { System.assert(false, 'OBS|np_3_ot_dp|' + new PubInner().ob(3, 'OuterTest.PubInner', 'Deeper')); }
  @IsTest static void m_np_4_ot_dp() { System.assert(false, 'OBS|np_4_ot_dp|' + new PubInner().ob(4, 'OuterTest.PubInner', 'Deeper')); }
  @IsTest static void m_np_5_ot_dp() { System.assert(false, 'OBS|np_5_ot_dp|' + new PubInner().ob(5, 'OuterTest.PubInner', 'Deeper')); }
  @IsTest static void m_np_6_ot_dp() { System.assert(false, 'OBS|np_6_ot_dp|' + new PubInner().ob(6, 'OuterTest.PubInner', 'Deeper')); }
  @IsTest static void m_nq_1_ot_pr() { System.assert(false, 'OBS|nq_1_ot_pr|' + new PrivInner().ob(1, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_nq_2_ot_pr() { System.assert(false, 'OBS|nq_2_ot_pr|' + new PrivInner().ob(2, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_nq_3_ot_pr() { System.assert(false, 'OBS|nq_3_ot_pr|' + new PrivInner().ob(3, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_nq_4_ot_pr() { System.assert(false, 'OBS|nq_4_ot_pr|' + new PrivInner().ob(4, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_nq_5_ot_pr() { System.assert(false, 'OBS|nq_5_ot_pr|' + new PrivInner().ob(5, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_nq_6_ot_pr() { System.assert(false, 'OBS|nq_6_ot_pr|' + new PrivInner().ob(6, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_nq_1_ot_df() { System.assert(false, 'OBS|nq_1_ot_df|' + new PrivInner().ob(1, 'OuterTest', 'DefInner')); }
  @IsTest static void m_nq_2_ot_df() { System.assert(false, 'OBS|nq_2_ot_df|' + new PrivInner().ob(2, 'OuterTest', 'DefInner')); }
  @IsTest static void m_nq_3_ot_df() { System.assert(false, 'OBS|nq_3_ot_df|' + new PrivInner().ob(3, 'OuterTest', 'DefInner')); }
  @IsTest static void m_nq_4_ot_df() { System.assert(false, 'OBS|nq_4_ot_df|' + new PrivInner().ob(4, 'OuterTest', 'DefInner')); }
  @IsTest static void m_nq_5_ot_df() { System.assert(false, 'OBS|nq_5_ot_df|' + new PrivInner().ob(5, 'OuterTest', 'DefInner')); }
  @IsTest static void m_nq_6_ot_df() { System.assert(false, 'OBS|nq_6_ot_df|' + new PrivInner().ob(6, 'OuterTest', 'DefInner')); }
  @IsTest static void m_nq_1_ot_tv() { System.assert(false, 'OBS|nq_1_ot_tv|' + new PrivInner().ob(1, 'OuterTest', 'TvInner')); }
  @IsTest static void m_nq_2_ot_tv() { System.assert(false, 'OBS|nq_2_ot_tv|' + new PrivInner().ob(2, 'OuterTest', 'TvInner')); }
  @IsTest static void m_nq_3_ot_tv() { System.assert(false, 'OBS|nq_3_ot_tv|' + new PrivInner().ob(3, 'OuterTest', 'TvInner')); }
  @IsTest static void m_nq_4_ot_tv() { System.assert(false, 'OBS|nq_4_ot_tv|' + new PrivInner().ob(4, 'OuterTest', 'TvInner')); }
  @IsTest static void m_nq_5_ot_tv() { System.assert(false, 'OBS|nq_5_ot_tv|' + new PrivInner().ob(5, 'OuterTest', 'TvInner')); }
  @IsTest static void m_nq_6_ot_tv() { System.assert(false, 'OBS|nq_6_ot_tv|' + new PrivInner().ob(6, 'OuterTest', 'TvInner')); }
  @IsTest static void m_nq_1_ot_pu() { System.assert(false, 'OBS|nq_1_ot_pu|' + new PrivInner().ob(1, 'OuterTest', 'PubInner')); }
  @IsTest static void m_nq_2_ot_pu() { System.assert(false, 'OBS|nq_2_ot_pu|' + new PrivInner().ob(2, 'OuterTest', 'PubInner')); }
  @IsTest static void m_nq_3_ot_pu() { System.assert(false, 'OBS|nq_3_ot_pu|' + new PrivInner().ob(3, 'OuterTest', 'PubInner')); }
  @IsTest static void m_nq_4_ot_pu() { System.assert(false, 'OBS|nq_4_ot_pu|' + new PrivInner().ob(4, 'OuterTest', 'PubInner')); }
  @IsTest static void m_nq_5_ot_pu() { System.assert(false, 'OBS|nq_5_ot_pu|' + new PrivInner().ob(5, 'OuterTest', 'PubInner')); }
  @IsTest static void m_nq_6_ot_pu() { System.assert(false, 'OBS|nq_6_ot_pu|' + new PrivInner().ob(6, 'OuterTest', 'PubInner')); }
  @IsTest static void m_nq_1_ot_dp() { System.assert(false, 'OBS|nq_1_ot_dp|' + new PrivInner().ob(1, 'OuterTest.PubInner', 'Deeper')); }
  @IsTest static void m_nq_2_ot_dp() { System.assert(false, 'OBS|nq_2_ot_dp|' + new PrivInner().ob(2, 'OuterTest.PubInner', 'Deeper')); }
  @IsTest static void m_nq_3_ot_dp() { System.assert(false, 'OBS|nq_3_ot_dp|' + new PrivInner().ob(3, 'OuterTest.PubInner', 'Deeper')); }
  @IsTest static void m_nq_4_ot_dp() { System.assert(false, 'OBS|nq_4_ot_dp|' + new PrivInner().ob(4, 'OuterTest.PubInner', 'Deeper')); }
  @IsTest static void m_nq_5_ot_dp() { System.assert(false, 'OBS|nq_5_ot_dp|' + new PrivInner().ob(5, 'OuterTest.PubInner', 'Deeper')); }
  @IsTest static void m_nq_6_ot_dp() { System.assert(false, 'OBS|nq_6_ot_dp|' + new PrivInner().ob(6, 'OuterTest.PubInner', 'Deeper')); }
  @IsTest static void m_j_dir_ot_pr() { System.assert(false, 'OBS|j_dir_ot_pr|' + jd(OuterTest.PrivInner.class)); }
  @IsTest static void m_j_fn_ot_pr() { System.assert(false, 'OBS|j_fn_ot_pr|' + jf('OuterTest.PrivInner')); }
  @IsTest static void m_j_dir_ot_df() { System.assert(false, 'OBS|j_dir_ot_df|' + jd(OuterTest.DefInner.class)); }
  @IsTest static void m_j_fn_ot_df() { System.assert(false, 'OBS|j_fn_ot_df|' + jf('OuterTest.DefInner')); }
  @IsTest static void m_j_dir_ot_tv() { System.assert(false, 'OBS|j_dir_ot_tv|' + jd(OuterTest.TvInner.class)); }
  @IsTest static void m_j_fn_ot_tv() { System.assert(false, 'OBS|j_fn_ot_tv|' + jf('OuterTest.TvInner')); }
  @IsTest static void m_j_dir_ot_pu() { System.assert(false, 'OBS|j_dir_ot_pu|' + jd(OuterTest.PubInner.class)); }
  @IsTest static void m_j_fn_ot_pu() { System.assert(false, 'OBS|j_fn_ot_pu|' + jf('OuterTest.PubInner')); }
  @IsTest static void m_si_1_ot_pr() { System.assert(false, 'OBS|si_1_ot_pr|' + String.valueOf(sinit.get('1_pr'))); }
  @IsTest static void m_ct_1_ot_pr() { System.assert(false, 'OBS|ct_1_ot_pr|' + String.valueOf(new OuterTest().cinit.get('1_pr'))); }
  @IsTest static void m_si_2_ot_pr() { System.assert(false, 'OBS|si_2_ot_pr|' + String.valueOf(sinit.get('2_pr'))); }
  @IsTest static void m_ct_2_ot_pr() { System.assert(false, 'OBS|ct_2_ot_pr|' + String.valueOf(new OuterTest().cinit.get('2_pr'))); }
  @IsTest static void m_si_3_ot_pr() { System.assert(false, 'OBS|si_3_ot_pr|' + String.valueOf(sinit.get('3_pr'))); }
  @IsTest static void m_ct_3_ot_pr() { System.assert(false, 'OBS|ct_3_ot_pr|' + String.valueOf(new OuterTest().cinit.get('3_pr'))); }
  @IsTest static void m_si_4_ot_pr() { System.assert(false, 'OBS|si_4_ot_pr|' + String.valueOf(sinit.get('4_pr'))); }
  @IsTest static void m_ct_4_ot_pr() { System.assert(false, 'OBS|ct_4_ot_pr|' + String.valueOf(new OuterTest().cinit.get('4_pr'))); }
  @IsTest static void m_si_5_ot_pr() { System.assert(false, 'OBS|si_5_ot_pr|' + String.valueOf(sinit.get('5_pr'))); }
  @IsTest static void m_ct_5_ot_pr() { System.assert(false, 'OBS|ct_5_ot_pr|' + String.valueOf(new OuterTest().cinit.get('5_pr'))); }
  @IsTest static void m_si_6_ot_pr() { System.assert(false, 'OBS|si_6_ot_pr|' + String.valueOf(sinit.get('6_pr'))); }
  @IsTest static void m_ct_6_ot_pr() { System.assert(false, 'OBS|ct_6_ot_pr|' + String.valueOf(new OuterTest().cinit.get('6_pr'))); }
  @IsTest static void m_si_1_ot_df() { System.assert(false, 'OBS|si_1_ot_df|' + String.valueOf(sinit.get('1_df'))); }
  @IsTest static void m_ct_1_ot_df() { System.assert(false, 'OBS|ct_1_ot_df|' + String.valueOf(new OuterTest().cinit.get('1_df'))); }
  @IsTest static void m_si_2_ot_df() { System.assert(false, 'OBS|si_2_ot_df|' + String.valueOf(sinit.get('2_df'))); }
  @IsTest static void m_ct_2_ot_df() { System.assert(false, 'OBS|ct_2_ot_df|' + String.valueOf(new OuterTest().cinit.get('2_df'))); }
  @IsTest static void m_si_3_ot_df() { System.assert(false, 'OBS|si_3_ot_df|' + String.valueOf(sinit.get('3_df'))); }
  @IsTest static void m_ct_3_ot_df() { System.assert(false, 'OBS|ct_3_ot_df|' + String.valueOf(new OuterTest().cinit.get('3_df'))); }
  @IsTest static void m_si_4_ot_df() { System.assert(false, 'OBS|si_4_ot_df|' + String.valueOf(sinit.get('4_df'))); }
  @IsTest static void m_ct_4_ot_df() { System.assert(false, 'OBS|ct_4_ot_df|' + String.valueOf(new OuterTest().cinit.get('4_df'))); }
  @IsTest static void m_si_5_ot_df() { System.assert(false, 'OBS|si_5_ot_df|' + String.valueOf(sinit.get('5_df'))); }
  @IsTest static void m_ct_5_ot_df() { System.assert(false, 'OBS|ct_5_ot_df|' + String.valueOf(new OuterTest().cinit.get('5_df'))); }
  @IsTest static void m_si_6_ot_df() { System.assert(false, 'OBS|si_6_ot_df|' + String.valueOf(sinit.get('6_df'))); }
  @IsTest static void m_ct_6_ot_df() { System.assert(false, 'OBS|ct_6_ot_df|' + String.valueOf(new OuterTest().cinit.get('6_df'))); }
  @IsTest static void m_si_1_ot_tv() { System.assert(false, 'OBS|si_1_ot_tv|' + String.valueOf(sinit.get('1_tv'))); }
  @IsTest static void m_ct_1_ot_tv() { System.assert(false, 'OBS|ct_1_ot_tv|' + String.valueOf(new OuterTest().cinit.get('1_tv'))); }
  @IsTest static void m_si_2_ot_tv() { System.assert(false, 'OBS|si_2_ot_tv|' + String.valueOf(sinit.get('2_tv'))); }
  @IsTest static void m_ct_2_ot_tv() { System.assert(false, 'OBS|ct_2_ot_tv|' + String.valueOf(new OuterTest().cinit.get('2_tv'))); }
  @IsTest static void m_si_3_ot_tv() { System.assert(false, 'OBS|si_3_ot_tv|' + String.valueOf(sinit.get('3_tv'))); }
  @IsTest static void m_ct_3_ot_tv() { System.assert(false, 'OBS|ct_3_ot_tv|' + String.valueOf(new OuterTest().cinit.get('3_tv'))); }
  @IsTest static void m_si_4_ot_tv() { System.assert(false, 'OBS|si_4_ot_tv|' + String.valueOf(sinit.get('4_tv'))); }
  @IsTest static void m_ct_4_ot_tv() { System.assert(false, 'OBS|ct_4_ot_tv|' + String.valueOf(new OuterTest().cinit.get('4_tv'))); }
  @IsTest static void m_si_5_ot_tv() { System.assert(false, 'OBS|si_5_ot_tv|' + String.valueOf(sinit.get('5_tv'))); }
  @IsTest static void m_ct_5_ot_tv() { System.assert(false, 'OBS|ct_5_ot_tv|' + String.valueOf(new OuterTest().cinit.get('5_tv'))); }
  @IsTest static void m_si_6_ot_tv() { System.assert(false, 'OBS|si_6_ot_tv|' + String.valueOf(sinit.get('6_tv'))); }
  @IsTest static void m_ct_6_ot_tv() { System.assert(false, 'OBS|ct_6_ot_tv|' + String.valueOf(new OuterTest().cinit.get('6_tv'))); }
  @IsTest static void m_si_1_ot_pu() { System.assert(false, 'OBS|si_1_ot_pu|' + String.valueOf(sinit.get('1_pu'))); }
  @IsTest static void m_ct_1_ot_pu() { System.assert(false, 'OBS|ct_1_ot_pu|' + String.valueOf(new OuterTest().cinit.get('1_pu'))); }
  @IsTest static void m_si_2_ot_pu() { System.assert(false, 'OBS|si_2_ot_pu|' + String.valueOf(sinit.get('2_pu'))); }
  @IsTest static void m_ct_2_ot_pu() { System.assert(false, 'OBS|ct_2_ot_pu|' + String.valueOf(new OuterTest().cinit.get('2_pu'))); }
  @IsTest static void m_si_3_ot_pu() { System.assert(false, 'OBS|si_3_ot_pu|' + String.valueOf(sinit.get('3_pu'))); }
  @IsTest static void m_ct_3_ot_pu() { System.assert(false, 'OBS|ct_3_ot_pu|' + String.valueOf(new OuterTest().cinit.get('3_pu'))); }
  @IsTest static void m_si_4_ot_pu() { System.assert(false, 'OBS|si_4_ot_pu|' + String.valueOf(sinit.get('4_pu'))); }
  @IsTest static void m_ct_4_ot_pu() { System.assert(false, 'OBS|ct_4_ot_pu|' + String.valueOf(new OuterTest().cinit.get('4_pu'))); }
  @IsTest static void m_si_5_ot_pu() { System.assert(false, 'OBS|si_5_ot_pu|' + String.valueOf(sinit.get('5_pu'))); }
  @IsTest static void m_ct_5_ot_pu() { System.assert(false, 'OBS|ct_5_ot_pu|' + String.valueOf(new OuterTest().cinit.get('5_pu'))); }
  @IsTest static void m_si_6_ot_pu() { System.assert(false, 'OBS|si_6_ot_pu|' + String.valueOf(sinit.get('6_pu'))); }
  @IsTest static void m_ct_6_ot_pu() { System.assert(false, 'OBS|ct_6_ot_pu|' + String.valueOf(new OuterTest().cinit.get('6_pu'))); }
}
