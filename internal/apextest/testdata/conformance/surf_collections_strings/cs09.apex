List<String> rows = new List<String>();
try { rows.add('objJoin=' + String.join(new List<Object>{1, null, true}, '|')); } catch (Exception e) { rows.add('objJoin=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('intNull=' + Integer.valueOf(null)); } catch (Exception e) { rows.add('intNull=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('decNull=' + Decimal.valueOf(null)); } catch (Exception e) { rows.add('decNull=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('fmtNull=' + String.format(null, new List<Object>{'x'})); } catch (Exception e) { rows.add('fmtNull=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('fmtArgsNull=' + String.format('{0}', null)); } catch (Exception e) { rows.add('fmtArgsNull=' + e.getTypeName() + '|' + e.getMessage()); }
try { List<String> l = new List<String>{'a'}; rows.add('sortNull=' + l.sort()); } catch (Exception e) { rows.add('sortNull=' + e.getTypeName() + '|' + e.getMessage()); }
for (String r : rows) { System.debug('OBS|' + r); }
