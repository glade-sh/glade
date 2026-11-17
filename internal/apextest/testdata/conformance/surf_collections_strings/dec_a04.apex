List<String> out = new List<String>();
Map<Decimal, String> m = new Map<Decimal, String>();
m.put(1.5, 'one_five');
out.add('A04\tMap\t1.5\tmap_lookup\t' + String.valueOf(m.get(1.50)) + '\t\t\t\t' + m.size() + '\t'
    + String.valueOf(m.containsKey(Decimal.valueOf('1.500'))) + '\tget_1.50');
out.add('END');
System.debug(String.join(out, '
'));
