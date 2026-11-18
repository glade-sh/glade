List<String> rows = new List<String>();
try { rows.add('p0=' + String.format('{0}', new List<Object>{'x'})); } catch (Exception e) { rows.add('p0=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('n2=' + String.format('{0,number,#.##}', new List<Object>{1.234})); } catch (Exception e) { rows.add('n2=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('n3=' + String.format('{0,number,#,##0.00}', new List<Object>{1234.5})); } catch (Exception e) { rows.add('n3=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('n0=' + String.format('{0,number}', new List<Object>{1234.5})); } catch (Exception e) { rows.add('n0=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('nd=' + String.format('{0,number,000}', new List<Object>{7})); } catch (Exception e) { rows.add('nd=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('date=' + String.format('{0,date}', new List<Object>{Date.newInstance(2026, 3, 8)})); } catch (Exception e) { rows.add('date=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('quote=' + String.format('It\'s {0}', new List<Object>{'x'})); } catch (Exception e) { rows.add('quote=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('twoQuote=' + String.format('a\'b {0}', new List<Object>{'x'})); } catch (Exception e) { rows.add('twoQuote=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('missing=' + String.format('{0} {1}', new List<Object>{'x'})); } catch (Exception e) { rows.add('missing=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('unclosed=' + String.format('{0', new List<Object>{'x'})); } catch (Exception e) { rows.add('unclosed=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('extra=' + String.format('{0}', new List<Object>{'a', 'b'})); } catch (Exception e) { rows.add('extra=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('nullarg=' + String.format('{0}', new List<Object>{null})); } catch (Exception e) { rows.add('nullarg=' + e.getTypeName() + '|' + e.getMessage()); }
for (String r : rows) { System.debug('OBS|' + r); }
