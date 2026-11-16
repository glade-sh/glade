List<String> rows = new List<String>();
rows.add('plain=' + JsonProbe.plainClass());
rows.add('nulls=' + JsonProbe.plainNulls());
rows.add('derived=' + JsonProbe.derived());
rows.add('transient=' + JsonProbe.transientOnly());
rows.add('transientPretty=' + JsonProbe.transientPretty().replace('\n', '\\n'));
rows.add('nested=' + JsonProbe.nested());
rows.add('nestedSupp=' + JsonProbe.nestedSuppressed());
try { rows.add('cyclic=' + JsonProbe.cyclic()); } catch (Exception e) { rows.add('cyclic=' + e.getTypeName() + '|' + e.getMessage()); }
for (String r : rows) { System.debug('OBS|' + r); }
