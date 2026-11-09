@IsTest private class V035OwnProbeT {
  @IsTest static void m_a01() { V035OwnPub.Pn v = new V035OwnPub.Pn(); System.assert(false, 'OBS|a01|' + String.valueOf(v.y())); }
  @IsTest static void m_a02() { gladens004.V035OwnPub.Pn v = new gladens004.V035OwnPub.Pn(); System.assert(false, 'OBS|a02|' + String.valueOf(v.y())); }
  @IsTest static void m_a03() { System.assert(false, 'OBS|a03|' + String.valueOf(V035OwnPub.x())); }
  @IsTest static void m_a04() { System.assert(false, 'OBS|a04|' + String.valueOf(gladens004.V035OwnPub.x())); }
  @IsTest static void m_a05() { V035OwnGlob.Gn v = new V035OwnGlob.Gn(); System.assert(false, 'OBS|a05|' + String.valueOf(v.y())); }
  @IsTest static void m_a06() { gladens004.V035OwnGlob.Gn v = new gladens004.V035OwnGlob.Gn(); System.assert(false, 'OBS|a06|' + String.valueOf(v.y())); }
  @IsTest static void m_a07() { V035OwnGlob.Pn2 v = new V035OwnGlob.Pn2(); System.assert(false, 'OBS|a07|' + String.valueOf(v.y())); }
  @IsTest static void m_a08() { gladens004.V035OwnGlob.Pn2 v = new gladens004.V035OwnGlob.Pn2(); System.assert(false, 'OBS|a08|' + String.valueOf(v.y())); }
  @IsTest static void m_a09() { V035OwnPub p = new V035OwnPub(); System.assert(false, 'OBS|a09|' + String.valueOf(p.inst())); }
  @IsTest static void m_a10() { gladens004.V035OwnPub p = new gladens004.V035OwnPub(); System.assert(false, 'OBS|a10|' + String.valueOf(p.inst())); }
  @IsTest static void m_a11() { V035OwnEnum e = V035OwnEnum.B; System.assert(false, 'OBS|a11|' + String.valueOf(e)); }
  @IsTest static void m_a12() { gladens004.V035OwnEnum e = gladens004.V035OwnEnum.B; System.assert(false, 'OBS|a12|' + String.valueOf(e)); }
  @IsTest static void m_a15() { System.assert(false, 'OBS|a15|' + String.valueOf(Type.forName('V035OwnPub') == null ? 'null' : Type.forName('V035OwnPub').getName())); }
  @IsTest static void m_a16() { System.assert(false, 'OBS|a16|' + String.valueOf(Type.forName('gladens004.V035OwnPub') == null ? 'null' : Type.forName('gladens004.V035OwnPub').getName())); }
  @IsTest static void m_a17() { System.assert(false, 'OBS|a17|' + String.valueOf(Type.forName('gladens004', 'V035OwnPub') == null ? 'null' : Type.forName('gladens004', 'V035OwnPub').getName())); }
  @IsTest static void m_a18() { System.assert(false, 'OBS|a18|' + String.valueOf(Type.forName('V035OwnPub.Pn') == null ? 'null' : Type.forName('V035OwnPub.Pn').getName())); }
  @IsTest static void m_a19() { System.assert(false, 'OBS|a19|' + String.valueOf(Type.forName('gladens004.V035OwnPub.Pn') == null ? 'null' : Type.forName('gladens004.V035OwnPub.Pn').getName())); }
  @IsTest static void m_a20() { System.assert(false, 'OBS|a20|' + String.valueOf(Type.forName('V035OwnPub.Pv') == null ? 'null' : Type.forName('V035OwnPub.Pv').getName())); }
  @IsTest static void m_a21() { v035ownpub.pn v = new v035ownpub.pn(); System.assert(false, 'OBS|a21|' + String.valueOf(v.y())); }
  @IsTest static void m_a22() { GLADENS004.v035ownpub.PN v = new GLADENS004.v035ownpub.PN(); System.assert(false, 'OBS|a22|' + String.valueOf(v.y())); }
  @IsTest static void m_a23() { Type t = V035OwnPub.class; System.assert(false, 'OBS|a23|' + String.valueOf(t.getName())); }
  @IsTest static void m_a24() { Type t = gladens004.V035OwnPub.Pn.class; System.assert(false, 'OBS|a24|' + String.valueOf(t.getName())); }
  @IsTest static void m_a25() { System.assert(false, 'OBS|a25|' + String.valueOf(String.valueOf(new V035OwnPub.Pn()).substringBefore(':'))); }
  @IsTest static void m_a26() { Object o = new V035OwnPub.Pn(); System.assert(false, 'OBS|a26|' + String.valueOf(o instanceof V035OwnPub.Pn)); }
  @IsTest static void m_a32() { Object o = new V035OwnPub(); System.assert(false, 'OBS|a32|' + String.valueOf(String.valueOf(o).substringBefore(':'))); }
  @IsTest static void m_a33() { System.assert(false, 'OBS|a33|' + String.valueOf(V035OwnPub.Pn.class.getName())); }
  @IsTest static void m_a34() { V035OwnIface i; System.assert(false, 'OBS|a34|' + String.valueOf('ok')); }
  @IsTest static void m_a35() { System.assert(false, 'OBS|a35|' + String.valueOf(Type.forName('V035OwnGlob.Pn2') == null ? 'null' : Type.forName('V035OwnGlob.Pn2').getName())); }
}
