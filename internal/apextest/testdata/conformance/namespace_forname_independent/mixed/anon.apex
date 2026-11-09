for (String t : new List<String>{'T24','T62'}) { for (String k : new List<String>{'PrivInner','DefInner','TvInner','PubInner'}) {
  Type x = Type.forName(t + '.' + k); System.debug('OBS|an_' + t + '_' + k + '|' + (x == null ? 'null' : x.getName()));
  System.debug('OBS|n24_' + t + '_' + k + '|' + N24.go(t + '.' + k)); } }
