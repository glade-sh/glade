List<String> rows = new List<String>();
try { rows.add('dtmb=' + Datetime.newInstanceGmt(2026, 1, 31, 12, 0, 0).monthsBetween(Datetime.newInstanceGmt(2026, 2, 28, 12, 0, 0))); } catch (Exception e) { rows.add('dtmb=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('dl=' + Date.newInstance(2026, 3, 8).formatLong()); } catch (Exception e) { rows.add('dl=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('dtms=' + Datetime.newInstanceGmt(2026, 3, 8, 10, 30, 6, 0).millisecond()); } catch (Exception e) { rows.add('dtms=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('dtaddm=' + Datetime.newInstanceGmt(2026, 1, 31, 12, 0, 0).addMonths(1)); } catch (Exception e) { rows.add('dtaddm=' + e.getTypeName() + '|' + e.getMessage()); }
for (String r : rows) { System.debug('OBS|' + r); }
