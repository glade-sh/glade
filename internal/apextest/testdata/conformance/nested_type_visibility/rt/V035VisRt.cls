@IsTest private class V035VisRt {
  @IsTest static void m_v07() { System.assert(false, 'OBS|v07|' + String.valueOf(V035Vis.inner())); }
  @IsTest static void m_v08() { System.assert(false, 'OBS|v08|' + String.valueOf(new V035Vis.Sib().z())); }
  @IsTest static void m_v13() { System.assert(false, 'OBS|v13|' + String.valueOf(Type.forName('V035Vis.Tv') == null ? 'null' : Type.forName('V035Vis.Tv').getName())); }
  @IsTest static void m_v14() { System.assert(false, 'OBS|v14|' + String.valueOf(Type.forName('V035Vis.Dn') == null ? 'null' : Type.forName('V035Vis.Dn').getName())); }
  @IsTest static void m_v15() { System.assert(false, 'OBS|v15|' + String.valueOf(Type.forName('V035Vis.Tdn') == null ? 'null' : Type.forName('V035Vis.Tdn').getName())); }
  @IsTest static void m_v17() { V035Vis.Pb v = new V035Vis.Pb(); System.assert(false, 'OBS|v17|' + String.valueOf(v.y())); }
  @IsTest static void m_v18() { System.assert(false, 'OBS|v18|' + String.valueOf(Type.forName('V035Vis.Pv') == null ? 'null' : Type.forName('V035Vis.Pv').getName())); }
  @IsTest static void m_v20() { System.assert(false, 'OBS|v20|' + String.valueOf(V035Vis.fn('V035Vis.Pv'))); }
  @IsTest static void m_v21() { System.assert(false, 'OBS|v21|' + String.valueOf(V035Vis.fn('V035Vis.Dn'))); }
  @IsTest static void m_v22() { System.assert(false, 'OBS|v22|' + String.valueOf(V035Vis.fn('V035Vis.Tv'))); }
  @IsTest static void m_v23() { System.assert(false, 'OBS|v23|' + String.valueOf(V035Vis.fn('V035Vis.Pb'))); }
  @IsTest static void m_v24() { System.assert(false, 'OBS|v24|' + String.valueOf(V035Vis.fn('V035Vis.Pen'))); }
}
