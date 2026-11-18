List<String> out = new List<String>();
List<Decimal> l1 = new List<Decimal>{ 1.5 };
List<Decimal> l2 = new List<Decimal>{ 1.50 };
out.add('A03\tList\t1.5\tlist_hash\t\t\t\t\t' + String.valueOf(l1.hashCode()) + '\t\t\t');
out.add('A03\tList\t1.50\tlist_hash\t\t\t\t\t' + String.valueOf(l2.hashCode()) + '\t\t\tl1_eq_l2=' + (l1 == l2));
out.add('END');
System.debug(String.join(out, '
'));
