List<String> r = new List<String>();
for (String k : new List<String>{'Pb','Pv','Dn','Tv'}) {
  r.add('OBS|d1_' + k + '|ns,Outer.k|' + NsVis.fn('gladens004', 'NsVis.' + k));
  r.add('OBS|d2_' + k + '|ns.Outer.k|' + NsVis.f1('gladens004.NsVis.' + k));
  r.add('OBS|d3_' + k + '|null,Outer.k|' + NsVis.fn(null, 'NsVis.' + k));
  r.add('OBS|d4_' + k + '|Outer.k|' + NsVis.f1('NsVis.' + k));
  r.add('OBS|d5_' + k + '|direct ns,Outer.k|' + (Type.forName('gladens004', 'NsVis.' + k) == null ? 'null' : 'found'));
  r.add('OBS|d8_' + k + '|inst ns.Outer.k|' + NsVis.inst('gladens004.NsVis.' + k));
}
System.debug(String.join(r, '\n'));
