List<String> rows = new List<String>();
try { rows.add('nulls=' + String.join(new List<String>{'a', null, 'b'}, ',')); } catch (Exception e) { rows.add('nulls=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('empty=' + String.join(new List<String>(), ',')); } catch (Exception e) { rows.add('empty=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('nullsep=' + String.join(new List<String>{'only'}, null)); } catch (Exception e) { rows.add('nullsep=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('set=' + String.join(new Set<String>{'b', 'a'}, '-')); } catch (Exception e) { rows.add('set=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('one=' + String.join(new List<String>{'only'}, ',')); } catch (Exception e) { rows.add('one=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('emptySep=' + String.join(new List<String>{'a', 'b'}, '')); } catch (Exception e) { rows.add('emptySep=' + e.getTypeName() + '|' + e.getMessage()); }
System.debug('OBS|' + String.join(rows, ';'));
