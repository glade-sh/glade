@IsTest
private class OtherTest {
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
  @IsTest static void m_x_1_ot_pr() { System.assert(false, 'OBS|x_1_ot_pr|' + ob_s(1, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_x_2_ot_pr() { System.assert(false, 'OBS|x_2_ot_pr|' + ob_s(2, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_x_3_ot_pr() { System.assert(false, 'OBS|x_3_ot_pr|' + ob_s(3, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_x_4_ot_pr() { System.assert(false, 'OBS|x_4_ot_pr|' + ob_s(4, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_x_5_ot_pr() { System.assert(false, 'OBS|x_5_ot_pr|' + ob_s(5, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_x_6_ot_pr() { System.assert(false, 'OBS|x_6_ot_pr|' + ob_s(6, 'OuterTest', 'PrivInner')); }
  @IsTest static void m_x_1_ot_df() { System.assert(false, 'OBS|x_1_ot_df|' + ob_s(1, 'OuterTest', 'DefInner')); }
  @IsTest static void m_x_2_ot_df() { System.assert(false, 'OBS|x_2_ot_df|' + ob_s(2, 'OuterTest', 'DefInner')); }
  @IsTest static void m_x_3_ot_df() { System.assert(false, 'OBS|x_3_ot_df|' + ob_s(3, 'OuterTest', 'DefInner')); }
  @IsTest static void m_x_4_ot_df() { System.assert(false, 'OBS|x_4_ot_df|' + ob_s(4, 'OuterTest', 'DefInner')); }
  @IsTest static void m_x_5_ot_df() { System.assert(false, 'OBS|x_5_ot_df|' + ob_s(5, 'OuterTest', 'DefInner')); }
  @IsTest static void m_x_6_ot_df() { System.assert(false, 'OBS|x_6_ot_df|' + ob_s(6, 'OuterTest', 'DefInner')); }
  @IsTest static void m_x_1_ot_tv() { System.assert(false, 'OBS|x_1_ot_tv|' + ob_s(1, 'OuterTest', 'TvInner')); }
  @IsTest static void m_x_2_ot_tv() { System.assert(false, 'OBS|x_2_ot_tv|' + ob_s(2, 'OuterTest', 'TvInner')); }
  @IsTest static void m_x_3_ot_tv() { System.assert(false, 'OBS|x_3_ot_tv|' + ob_s(3, 'OuterTest', 'TvInner')); }
  @IsTest static void m_x_4_ot_tv() { System.assert(false, 'OBS|x_4_ot_tv|' + ob_s(4, 'OuterTest', 'TvInner')); }
  @IsTest static void m_x_5_ot_tv() { System.assert(false, 'OBS|x_5_ot_tv|' + ob_s(5, 'OuterTest', 'TvInner')); }
  @IsTest static void m_x_6_ot_tv() { System.assert(false, 'OBS|x_6_ot_tv|' + ob_s(6, 'OuterTest', 'TvInner')); }
  @IsTest static void m_x_1_ot_pu() { System.assert(false, 'OBS|x_1_ot_pu|' + ob_s(1, 'OuterTest', 'PubInner')); }
  @IsTest static void m_x_2_ot_pu() { System.assert(false, 'OBS|x_2_ot_pu|' + ob_s(2, 'OuterTest', 'PubInner')); }
  @IsTest static void m_x_3_ot_pu() { System.assert(false, 'OBS|x_3_ot_pu|' + ob_s(3, 'OuterTest', 'PubInner')); }
  @IsTest static void m_x_4_ot_pu() { System.assert(false, 'OBS|x_4_ot_pu|' + ob_s(4, 'OuterTest', 'PubInner')); }
  @IsTest static void m_x_5_ot_pu() { System.assert(false, 'OBS|x_5_ot_pu|' + ob_s(5, 'OuterTest', 'PubInner')); }
  @IsTest static void m_x_6_ot_pu() { System.assert(false, 'OBS|x_6_ot_pu|' + ob_s(6, 'OuterTest', 'PubInner')); }
  @IsTest static void m_x_1_ou_pr() { System.assert(false, 'OBS|x_1_ou_pr|' + ob_s(1, 'OuterMain', 'PrivInner')); }
  @IsTest static void m_x_2_ou_pr() { System.assert(false, 'OBS|x_2_ou_pr|' + ob_s(2, 'OuterMain', 'PrivInner')); }
  @IsTest static void m_x_3_ou_pr() { System.assert(false, 'OBS|x_3_ou_pr|' + ob_s(3, 'OuterMain', 'PrivInner')); }
  @IsTest static void m_x_4_ou_pr() { System.assert(false, 'OBS|x_4_ou_pr|' + ob_s(4, 'OuterMain', 'PrivInner')); }
  @IsTest static void m_x_5_ou_pr() { System.assert(false, 'OBS|x_5_ou_pr|' + ob_s(5, 'OuterMain', 'PrivInner')); }
  @IsTest static void m_x_6_ou_pr() { System.assert(false, 'OBS|x_6_ou_pr|' + ob_s(6, 'OuterMain', 'PrivInner')); }
  @IsTest static void m_x_1_ou_df() { System.assert(false, 'OBS|x_1_ou_df|' + ob_s(1, 'OuterMain', 'DefInner')); }
  @IsTest static void m_x_2_ou_df() { System.assert(false, 'OBS|x_2_ou_df|' + ob_s(2, 'OuterMain', 'DefInner')); }
  @IsTest static void m_x_3_ou_df() { System.assert(false, 'OBS|x_3_ou_df|' + ob_s(3, 'OuterMain', 'DefInner')); }
  @IsTest static void m_x_4_ou_df() { System.assert(false, 'OBS|x_4_ou_df|' + ob_s(4, 'OuterMain', 'DefInner')); }
  @IsTest static void m_x_5_ou_df() { System.assert(false, 'OBS|x_5_ou_df|' + ob_s(5, 'OuterMain', 'DefInner')); }
  @IsTest static void m_x_6_ou_df() { System.assert(false, 'OBS|x_6_ou_df|' + ob_s(6, 'OuterMain', 'DefInner')); }
  @IsTest static void m_x_1_ou_tv() { System.assert(false, 'OBS|x_1_ou_tv|' + ob_s(1, 'OuterMain', 'TvInner')); }
  @IsTest static void m_x_2_ou_tv() { System.assert(false, 'OBS|x_2_ou_tv|' + ob_s(2, 'OuterMain', 'TvInner')); }
  @IsTest static void m_x_3_ou_tv() { System.assert(false, 'OBS|x_3_ou_tv|' + ob_s(3, 'OuterMain', 'TvInner')); }
  @IsTest static void m_x_4_ou_tv() { System.assert(false, 'OBS|x_4_ou_tv|' + ob_s(4, 'OuterMain', 'TvInner')); }
  @IsTest static void m_x_5_ou_tv() { System.assert(false, 'OBS|x_5_ou_tv|' + ob_s(5, 'OuterMain', 'TvInner')); }
  @IsTest static void m_x_6_ou_tv() { System.assert(false, 'OBS|x_6_ou_tv|' + ob_s(6, 'OuterMain', 'TvInner')); }
  @IsTest static void m_x_1_ou_pu() { System.assert(false, 'OBS|x_1_ou_pu|' + ob_s(1, 'OuterMain', 'PubInner')); }
  @IsTest static void m_x_2_ou_pu() { System.assert(false, 'OBS|x_2_ou_pu|' + ob_s(2, 'OuterMain', 'PubInner')); }
  @IsTest static void m_x_3_ou_pu() { System.assert(false, 'OBS|x_3_ou_pu|' + ob_s(3, 'OuterMain', 'PubInner')); }
  @IsTest static void m_x_4_ou_pu() { System.assert(false, 'OBS|x_4_ou_pu|' + ob_s(4, 'OuterMain', 'PubInner')); }
  @IsTest static void m_x_5_ou_pu() { System.assert(false, 'OBS|x_5_ou_pu|' + ob_s(5, 'OuterMain', 'PubInner')); }
  @IsTest static void m_x_6_ou_pu() { System.assert(false, 'OBS|x_6_ou_pu|' + ob_s(6, 'OuterMain', 'PubInner')); }
  @IsTest static void m_jx_fn_ot_pr() { System.assert(false, 'OBS|jx_fn_ot_pr|' + jf('OuterTest.PrivInner')); }
  @IsTest static void m_jx_fn_ot_df() { System.assert(false, 'OBS|jx_fn_ot_df|' + jf('OuterTest.DefInner')); }
  @IsTest static void m_jx_fn_ot_tv() { System.assert(false, 'OBS|jx_fn_ot_tv|' + jf('OuterTest.TvInner')); }
  @IsTest static void m_jx_fn_ot_pu() { System.assert(false, 'OBS|jx_fn_ot_pu|' + jf('OuterTest.PubInner')); }
}
