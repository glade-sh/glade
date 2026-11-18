List<String> rows = new List<String>();
try { rows.add('dmb=' + Date.newInstance(2026, 1, 31).monthsBetween(Date.newInstance(2026, 2, 28))); } catch (Exception e) { rows.add('dmb=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('dmb2=' + Date.newInstance(2026, 1, 1).monthsBetween(Date.newInstance(2026, 1, 16))); } catch (Exception e) { rows.add('dmb2=' + e.getTypeName() + '|' + e.getMessage()); }
try {
 Object o = Date.newInstance(2026, 1, 31).addMonths(1);
 rows.add('addmType=' + (o instanceof Date) + ',' + (o instanceof Datetime) + ',v=' + String.valueOf(o));
} catch (Exception e) { rows.add('addmType=' + e.getTypeName() + '|' + e.getMessage()); }
for (String r : rows) { System.debug('OBS|' + r); }
