List<String> a = new List<String>{'b', null, 'A', 'a', 'B', null, 'b'};
List<String> c = a.clone();
String s1;
try { c.sort(); s1 = String.join(c, ',') + '|firstNull=' + (c[0] == null); } catch (Exception e) { s1 = e.getTypeName() + '|' + e.getMessage(); }
List<Integer> n = new List<Integer>{3, null, 1, -2};
String s2;
try { n.sort(); s2 = String.join(n, ','); } catch (Exception e) { s2 = e.getTypeName() + '|' + e.getMessage(); }
List<Decimal> d = new List<Decimal>{1.5, null, -1, 1.50};
String s3;
try { d.sort(); s3 = String.join(d, ','); } catch (Exception e) { s3 = e.getTypeName() + '|' + e.getMessage(); }
List<Account> so = new List<Account>{new Account(Name='b'), null, new Account(Name='A')};
String s4;
try { so.sort(); s4 = String.valueOf(so.size()); } catch (Exception e) { s4 = e.getTypeName() + '|' + e.getMessage(); }
System.debug('OBS|str=' + s1);
System.debug('OBS|int=' + s2);
System.debug('OBS|dec=' + s3);
System.debug('OBS|sobj=' + s4);
