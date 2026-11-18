List<String> rows = new List<String>();
try { rows.add('time=' + JSON.serialize(Time.newInstance(12, 34, 56, 789))); } catch (Exception e) { rows.add('time=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('blob=' + JSON.serialize(Blob.valueOf('hello'))); } catch (Exception e) { rows.add('blob=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('long=' + JSON.serialize(2147483648L)); } catch (Exception e) { rows.add('long=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('max=' + JSON.serialize(9223372036854775807L)); } catch (Exception e) { rows.add('max=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('dec=' + JSON.serialize(Decimal.valueOf('12.50')) + '|scale=' + Decimal.valueOf('12.50').scale()); } catch (Exception e) { rows.add('dec=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('neg0=' + JSON.serialize(Decimal.valueOf('-0.00'))); } catch (Exception e) { rows.add('neg0=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('date=' + JSON.serialize(Date.newInstance(2026, 1, 2))); } catch (Exception e) { rows.add('date=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('dt=' + JSON.serialize(Datetime.newInstanceGmt(2026, 1, 2, 3, 4, 5))); } catch (Exception e) { rows.add('dt=' + e.getTypeName() + '|' + e.getMessage()); }
for (String r : rows) { System.debug('OBS|' + r); }
