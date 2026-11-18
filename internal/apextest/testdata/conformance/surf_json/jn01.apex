List<String> rows = new List<String>();
try { rows.add('int=' + JSON.serialize(new Map<Integer,String>{10=>'c', 1=>'a', 2=>'b'})); } catch (Exception e) { rows.add('int=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('long=' + JSON.serialize(new Map<Long,String>{3000000000L=>'big', 1L=>'one'})); } catch (Exception e) { rows.add('long=' + e.getTypeName() + '|' + e.getMessage()); }
try { rows.add('dec=' + JSON.serialize(new Map<Decimal,String>{1.0=>'one', 1.5=>'half', -2.50=>'neg'})); } catch (Exception e) { rows.add('dec=' + e.getTypeName() + '|' + e.getMessage()); }
try {
 Map<Object,String> mix = new Map<Object,String>();
 mix.put(1, 'int');
 mix.put('1', 'str');
 mix.put(true, 'bool');
 mix.put(null, 'nil');
 rows.add('mix=' + JSON.serialize(mix));
} catch (Exception e) { rows.add('mix=' + e.getTypeName() + '|' + e.getMessage()); }
try {
 Map<SObject,Integer> sk = new Map<SObject,Integer>();
 sk.put(new Account(Name='K'), 1);
 rows.add('sobj=' + JSON.serialize(sk));
} catch (Exception e) { rows.add('sobj=' + e.getTypeName() + '|' + e.getMessage()); }
try {
 Account a = new Account(Name='IdKeys');
 insert a;
 Map<Id,String> ik = new Map<Id,String>();
 ik.put(a.Id, 'one');
 rows.add('id=' + JSON.serialize(ik));
} catch (Exception e) { rows.add('id=' + e.getTypeName() + '|' + e.getMessage()); }
try {
 Object back = JSON.deserializeUntyped(JSON.serialize(new Map<Integer,String>{10=>'c', 1=>'a'}));
 rows.add('back=' + JSON.serialize(back));
} catch (Exception e) { rows.add('back=' + e.getTypeName() + '|' + e.getMessage()); }
for (String r : rows) { System.debug('OBS|' + r); }
