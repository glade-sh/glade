List<String> rows = new List<String>();
try { rows.add('aEmpty=' + JSON.deserialize('', Account.class)); } catch (Exception e) { rows.add('aEmpty=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('uEmpty=' + JSON.deserializeUntyped('')); } catch (Exception e) { rows.add('uEmpty=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('uSpace=' + JSON.deserializeUntyped('   ')); } catch (Exception e) { rows.add('uSpace=' + e.getTypeName() + '|' + e.getMessage()); }
try { List<Account> l = (List<Account>) JSON.deserialize('[]', List<Account>.class); rows.add('lEmpty=' + l.size()); } catch (Exception e) { rows.add('lEmpty=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('uNull=' + JSON.deserializeUntyped('null')); } catch (Exception e) { rows.add('uNull=' + e.getTypeName() + '|' + e.getMessage()); }
try {
 JSONParser p0 = JSON.createParser('');
 rows.add('pEmpty=' + p0.nextToken());
} catch (Exception e) { rows.add('pEmpty=' + e.getTypeName() + '|' + e.getMessage()); }
try {
 JSONParser p = JSON.createParser('{"a":[1,2],"b":3}');
 List<String> seq = new List<String>();
 while (p.nextToken() != null) {
  seq.add(String.valueOf(p.getCurrentToken()) + ':' + String.valueOf(p.getCurrentName()));
 }
 rows.add('tokens=' + String.join(seq, ','));
} catch (Exception e) { rows.add('tokens=' + e.getTypeName() + '|' + e.getMessage()); }
for (String r : rows) { System.debug('OBS|' + r); }
