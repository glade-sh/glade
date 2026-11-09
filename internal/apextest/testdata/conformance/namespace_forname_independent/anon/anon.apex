List<String> os = new List<String>{'OuterTest','OuterMain'};
List<String> ocs = new List<String>{'ot','ou'};
List<String> ks = new List<String>{'PrivInner','DefInner','TvInner','PubInner'};
List<String> kcs = new List<String>{'pr','df','tv','pu'};
for (Integer s = 1; s <= 6; s++) {
  for (Integer oi = 0; oi < 2; oi++) {
    for (Integer ki = 0; ki < 4; ki++) {
      String o = os[oi]; String k = ks[ki]; Type t; String r;
      try {
        if (s == 1) t = Type.forName('gladens004', o + '.' + k);
        else if (s == 2) t = Type.forName('gladens004.' + o + '.' + k);
        else if (s == 3) t = Type.forName(o + '.' + k);
        else if (s == 4) t = Type.forName(null, o + '.' + k);
        else if (s == 5) t = Type.forName('gladens004.' + o + '$' + k);
        else t = Type.forName('', o + '.' + k);
        if (t == null) { r = 'null'; }
        else {
          r = t.getName();
          try { Object x = t.newInstance(); r += '|ni=ok'; try { Callable c = (Callable) x; r += '|cast=ok:' + String.valueOf(c.call('go', null)); } catch (Exception e) { r += '|cast=' + e.getMessage(); } } catch (Exception e) { r += '|ni=' + e.getMessage(); }
        }
      } catch (Exception e) { r = ('EXC:' + e.getTypeName() + ':' + e.getMessage()); }
      System.debug('OBS|a_' + s + '_' + ocs[oi] + '_' + kcs[ki] + '|' + r.replace('\n', ' '));
    }
  }
}
